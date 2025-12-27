package agent

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

var restartEndpointServer chan struct{}

func Run() {
	plexus.SetLogging(Config.Verbosity)
	if err := boltdb.Initialize(Config.DataDir+"plexus-agent.db", []string{deviceTable, networkTable}); err != nil {
		slog.Error("failed to initialize database", "error", err)
		return
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	restartEndpointServer = make(chan struct{})
	self, err := newDevice()
	if err != nil {
		slog.Error("new device", "error", err)
	}
	ns, ec := startBroker()
	if err := connectToServer(self); err != nil {
		slog.Error("connect to server", "error", err)
	}
	startAllInterfaces(self)
	checkinTicker := time.NewTicker(checkinTime)
	serverTicker := time.NewTicker(serverCheckTime)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	go privateEndpointServer(ctx, wg)
	for {
		select {
		case <-quit:
			slog.Info("quit")
			cancel()
			slog.Info("deleting wg interfaces")
			deleteAllInterfaces()
			slog.Info("stopping tickers")
			checkinTicker.Stop()
			// serverTicker.Stop().
			closeServerConnections()
			slog.Info("shutdown nats server")
			_ = ec.Drain()
			go ns.Shutdown()
			slog.Info("wait for nat server shutdown to complete")
			ns.WaitForShutdown()
			slog.Info("nats server has shutdown")
			wg.Wait()
			slog.Info("exiting ...")
			return
		case <-checkinTicker.C:
			checkin()
		case <-serverTicker.C:
			// check server connection in case server was down when tried to connect earlier.
			slog.Debug("check server connection")
			if serverConn.Load() == nil {
				slog.Info("not connected to server.... retrying")
				if err := connectToServer(self); err != nil {
					slog.Error("server connection", "error", err)
				} else {
					slog.Info("connected to server")
				}
			}
		case <-restartEndpointServer:
			cancel()
			wg.Wait()
			wg.Add(1)
			ctx, cancel = context.WithCancel(context.Background())
			go privateEndpointServer(ctx, wg)
		}
	}
}

func connectToServer(self Device) error {
	kp, err := nkeys.FromSeed([]byte(self.Seed))
	if err != nil {
		return err
	}
	publicKey, err := kp.PublicKey()
	if err != nil {
		return err
	}
	sign := func(nonce []byte) ([]byte, error) {
		return kp.Sign(nonce)
	}
	opts := []nats.Option{nats.Name("plexus-agent " + self.Name)}
	opts = append(opts, []nats.Option{
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Info("disonnected from server", "error", err)
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			slog.Info("nats connection closed")
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			slog.Info("reconnected to nats server")
		}),
		nats.ErrorHandler(func(_ *nats.Conn, s *nats.Subscription, err error) {
			if s != nil {
				slog.Info("nats error", "subject", s.Subject, "error", err)
			} else {
				slog.Info("nats error", "error", err)
			}
		}),
		nats.Nkey(publicKey, sign),
	}...)
	slog.Debug("connecting to server", "url", self.Server)
	nc, err := nats.Connect(self.Server, opts...)
	if err != nil {
		return err
	}
	serverConn.Store(nc)
	subcribeToServerTopics(self)
	return nil
}

func closeServerConnections() {
	for _, sub := range subscriptions {
		if err := sub.Drain(); err != nil {
			slog.Error("drain subscription", "sub", sub.Subject, "error", err)
		}
	}
	ec := serverConn.Load()
	if ec != nil {
		ec.Close()
	}
}

func privateEndpointServer(ctx context.Context, wg *sync.WaitGroup) {
	slog.Debug("private endpoint server")
	defer wg.Done()
	networks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		return
	}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		return
	}
	for _, network := range networks {
		me := getSelfFromPeers(&self, network.Peers)
		if me.PrivateEndpoint == nil {
			continue
		}
		slog.Info("tcp listener starting on private endpoint", "endpoint", me.PrivateEndpoint, "port", me.ListenPort)
		listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: me.PrivateEndpoint, Port: me.ListenPort})
		if err != nil {
			slog.Error("public endpoint server", "error", err)
			return
		}
		go func() {
			for {
				select {
				case <-ctx.Done():
					slog.Debug("closing private endpoint server")
					listener.Close()
					return
				default:
					if err := listener.SetDeadline(time.Now().Add(endpointServerTimeout)); err != nil {
						slog.Error("set deadline", "error", err)
						continue
					}
					c, err := listener.Accept()
					if errors.Is(err, os.ErrDeadlineExceeded) {
						continue
					}
					if err != nil {
						slog.Warn("connect error", "error", err)
						continue
					}
					go handleConn(c, self.WGPublicKey)
				}
			}
		}()
	}
}

func handleConn(c net.Conn, reply string) {
	defer c.Close()
	reader := bufio.NewReader(c)
	_, err := reader.ReadBytes(byte('.'))
	if err != nil {
		return
	}
	_, _ = c.Write([]byte(reply))
}
