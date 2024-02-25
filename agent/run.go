package agent

import (
	"context"
	"fmt"
	"log"
	"log/slog"
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

var ()

func Run() {
	if err := plexus.WritePID(os.Getenv("HOME")+"/.cache/plexus-agent.pid", os.Getpid()); err != nil {
		slog.Error("failed to write pid to file", "error", err)
	}
	if err := boltdb.Initialize(os.Getenv("HOME")+"/.local/share/plexus/plexus-agent.db", []string{"devices", "networks"}); err != nil {
		slog.Error("failed to initialize database", "error", err)
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	fmt.Println(deleteAllInterface(&wg))
	quit := make(chan os.Signal, 1)
	reset := make(chan os.Signal, 1)
	restart = make(chan struct{}, 1)
	natsfail = make(chan struct{}, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	//signal.Notify(quit, syscall.SIGTERM)
	signal.Notify(reset, syscall.SIGHUP)
	ctx, cancel := context.WithCancel(context.Background())
	self, err := newDevice()
	if err != nil {
		slog.Error("new device", "error", err)
	}
	startAllInterfaces(self)
	wg.Add(1)
	go startAgentNatsServer(ctx, &wg)
	slog.Info("setup nats")
	connectToServers()
	//slog.Info("refresh data from servers")
	//refreshData(self)
	//slog.Info("set up subcriptions")
	//setupSubs(ctx, &wg, self)
	checkinTicker := time.NewTicker(checkinTime)
	serverTicker := time.NewTicker(serverCheckTime)
	for {
		select {
		case <-quit:
			slog.Info("quit")
			checkinTicker.Stop()
			serverTicker.Stop()
			cancel()
			wg.Wait()
			slog.Info("go routines stopped")
			os.Exit(1)
		case <-reset:
			slog.Info("reset")
			cancel()
			wg.Wait()
			slog.Info("go routines stopped by reset")
			ctx, cancel = context.WithCancel(context.Background())
			wg.Add(1)
			go startAgentNatsServer(ctx, &wg)
			connectToServers()
			//refreshData(self)
			closeServerConnections()
			connectToServers()
			//setupSubs(ctx, &wg, self)
		case <-restart:
			slog.Info("restart")
			cancel()
			wg.Wait()
			slog.Info("go routines stopped by restart")
			ctx, cancel = context.WithCancel(context.Background())
			wg.Add(1)
			go startAgentNatsServer(ctx, &wg)
			connectToServers()
			//refreshData(self)
			//setupSubs(ctx, &wg, self)
		case <-checkinTicker.C:
			wg.Add(1)
			checkin(&wg)
		case <-serverTicker.C:
			slog.Debug("refreshing server connection")
			closeServerConnections()
			connectToServers()
		}
	}
}

func connectToServer(self plexus.Device, server string) (*nats.EncodedConn, error) {
	kp, err := nkeys.FromSeed([]byte(self.Seed))
	if err != nil {
		return nil, err
	}
	pk, err := kp.PublicKey()
	if err != nil {
		return nil, err
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
	slog.Debug("connecting to server", "url", server)
	nc, err := nats.Connect(server, opts...)
	if err != nil {
		return nil, err
	}
	return nats.NewEncodedConn(nc, nats.JSON_ENCODER)
}

func checkin(wg *sync.WaitGroup) {
	defer wg.Done()
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("get device", "error", err)
		return
	}
	for server, data := range serverMap {
		if !data.EC.Conn.IsConnected() {
			slog.Debug("not connected to server broker .... skipping checkin", "server", server)
			continue
		}
		msg, err := data.EC.Conn.Request("checkin."+self.WGPublicKey, []byte("checking"), NatsTimeout)
		if err != nil {
			slog.Error("error publishing checkin ", "error", err)
			continue
		}
		log.Println("checkin response from server", server, string(msg.Data))
	}
	publishConnectivity(self)
}

func closeServerConnections() {
	for _, server := range serverMap {
		for _, sub := range server.Subscriptions {
			if err := sub.Drain(); err != nil {
				slog.Error("drain subscription", "sub", sub.Subject, "error", err)
			}
		}
		server.EC.Close()
	}
}
