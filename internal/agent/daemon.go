package agent

import (
	"bufio"
	"context"
	"fmt"
	"log"
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

func Run() {
	plexus.SetLogging(Config.Verbosity)
	if err := boltdb.Initialize(path+"plexus-agent.db", []string{deviceTable, networkTable}); err != nil {
		slog.Error("failed to initialize database", "error", err)
		return
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
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
			//serverTicker.Stop()
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
			// reconnect to servers in case server was down when tried to connect earlier
			slog.Debug("refreshing server connection")
			closeServerConnections()
			if err := connectToServer(self); err != nil {
				slog.Error("server connection", "error", err)
			}
		}
	}
}

func connectToServer(self Device) error {
	kp, err := nkeys.FromSeed([]byte(self.Seed))
	if err != nil {
		return err
	}
	pk, err := kp.PublicKey()
	if err != nil {
		return err
	}
	sign := func(nonce []byte) ([]byte, error) {
		return kp.Sign(nonce)
	}
	opts := []nats.Option{nats.Name("plexus-agent " + self.Name)}
	opts = append(opts, []nats.Option{
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(c *nats.Conn, err error) {
			slog.Info("disonnected from server", "error", err)
		}),
		nats.ClosedHandler(func(c *nats.Conn) {
			slog.Info("nats connection closed")
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			slog.Info("reconnected to nats server")
		}),
		nats.ErrorHandler(func(c *nats.Conn, s *nats.Subscription, err error) {
			if s != nil {
				slog.Info("nats error", "subject", s.Subject, "error", err)
			} else {
				slog.Info("nats error", "error", err)
			}
		}),
		nats.Nkey(pk, sign),
	}...)
	slog.Debug("connecting to server", "url", self.Server)
	nc, err := nats.Connect(self.Server, opts...)
	if err != nil {
		return err
	}
	serverEC, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return err
	}
	serverConn.Store(serverEC)
	subcribeToServerTopics(self)
	return nil
}

func checkin() {
	slog.Debug("checkin")
	checkinData := plexus.CheckinData{}
	serverResponse := plexus.MessageResponse{}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
		return
	}
	checkinData.ID = self.WGPublicKey
	checkinData.Version = self.Version
	checkinData.Endpoint = self.Endpoint
	serverEC := serverConn.Load()
	if serverEC == nil {
		slog.Debug("not connected to server broker .... skipping checkin")
		return
	}
	if !serverEC.Conn.IsConnected() {
		slog.Debug("not connected to server broker .... skipping checkin")
		return
	}
	checkinData.Connections = getConnectivity()
	if err := serverEC.Request(self.WGPublicKey+".checkin", checkinData, &serverResponse, NatsTimeout); err != nil {
		slog.Error("error publishing checkin ", "error", err)
		return
	}
	log.Println("checkin response from server", serverResponse.Message)
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
		slog.Info("tcp listener staating on prviate endpoint")
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", me.PrivateEndpoint, me.ListenPort))
		if err != nil {
			slog.Error("public endpoint server", "error", err)
			return
		}
		go func() {
			for {
				select {
				case <-ctx.Done():
					listener.Close()
					return
				default:
					c, err := listener.Accept()
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
