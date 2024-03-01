package agent

import (
	"log"
	"log/slog"
	"os"
	"os/signal"
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
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	self, err := newDevice()
	if err != nil {
		slog.Error("new device", "error", err)
	}
	changed, _, _ := stunCheck(self, self.ListenPort)
	startAllInterfaces(self)
	ns, ec := startAgentNatsServer()
	connectToServers()
	checkinTicker := time.NewTicker(checkinTime)
	if changed {
		slog.Info("stun change, sending update to servers")
		sendDeviceUpdate()
	}
	//serverTicker := time.NewTicker(serverCheckTime)
	for {
		select {
		case <-quit:
			slog.Info("quit")
			slog.Info("deleting wg interfaces")
			deleteAllInterfaces()
			slog.Info("stopping tickers")
			checkinTicker.Stop()
			//serverTicker.Stop()
			slog.Info("shutdown nats server")
			ec.Drain()
			go ns.Shutdown()
			slog.Info("wait for nat server shutdown to complete")
			ns.WaitForShutdown()
			slog.Info("nats server has shutdown")
			slog.Info("exiting ...")
			return
		case <-checkinTicker.C:
			checkin()
			//case <-serverTicker.C:
			// reconnect to servers in case server was down when tried to connect earlier
			//slog.Debug("refreshing server connection")
			//closeServerConnections()
			//connectToServers()
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

func checkin() {
	slog.Debug("checkin")
	checkinData := plexus.CheckinData{}
	serverResponse := plexus.NetworkResponse{}
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("get device", "error", err)
		return
	}
	stunCheck(self, checkPort(self.ListenPort))
	checkinData.ID = self.WGPublicKey
	checkinData.Version = self.Version
	checkinData.ListenPort = self.ListenPort
	checkinData.PublicListenPort = self.PublicListenPort
	checkinData.Endpoint = self.Endpoint

	for server, data := range serverMap {
		if !data.EC.Conn.IsConnected() {
			slog.Debug("not connected to server broker .... skipping checkin", "server", server)
			continue
		}
		checkinData.Connections = getConnectivity(server)
		if err := data.EC.Request("checkin."+self.WGPublicKey, checkinData, &serverResponse, NatsTimeout); err != nil {
			slog.Error("error publishing checkin ", "error", err)
			continue
		}
		log.Println("checkin response from server", server, serverResponse.Error, serverResponse.Message)
	}
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

func sendDeviceUpdate() {
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("get device", "error", err)
		return
	}
	for _, server := range self.Servers {
		conn, ok := serverMap[server]
		if !ok {
			slog.Error("server not mapped", "server", server)
			return
		}
		if err := conn.EC.Publish("update."+self.WGPublicKey, plexus.UpdateRequest{
			Action: plexus.UpdatePeer,
			Peer: plexus.Peer{
				WGPublicKey:      self.WGPublicKey,
				PubNkey:          self.PubNkey,
				Version:          self.Version,
				Name:             self.Name,
				OS:               self.OS,
				ListenPort:       self.ListenPort,
				PublicListenPort: self.PublicListenPort,
				Endpoint:         self.Endpoint,
				NatsConnected:    true,
			},
		}); err != nil {
			slog.Error("publish device update", "error", err)
		}
	}
}
