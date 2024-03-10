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
	serverMap = initServerMap()
	startAllInterfaces(self)
	ns, ec := startAgentNatsServer()
	connectToServers()
	checkinTicker := time.NewTicker(checkinTime)
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
			closeServerConnections()
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

func connectToServer(self Device, server string) (*nats.EncodedConn, error) {
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
	serverResponse := plexus.ServerResponse{}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
		return
	}
	//stunCheck(self, checkPort(self.ListenPort))
	checkinData.ID = self.WGPublicKey
	checkinData.Version = self.Version
	//checkinData.ListenPort = self.ListenPort
	//checkinData.PublicListenPort = self.PublicListenPort
	checkinData.Endpoint = self.Endpoint
	serverMap.mutex.RLock()
	defer serverMap.mutex.RUnlock()
	for server, data := range serverMap.data {
		if !data.EC.Conn.IsConnected() {
			slog.Debug("not connected to server broker .... skipping checkin", "server", server)
			continue
		}
		checkinData.Connections = getConnectivity(server)
		if err := data.EC.Request("update."+self.WGPublicKey, plexus.AgentRequest{
			Action:      plexus.Checkin,
			CheckinData: checkinData,
		}, &serverResponse, NatsTimeout); err != nil {
			slog.Error("error publishing checkin ", "error", err)
			continue
		}
		log.Println("checkin response from server", server, serverResponse.Error, serverResponse.Message)
	}
}

func closeServerConnections() {
	serverMap.mutex.RLock()
	defer serverMap.mutex.RUnlock()
	for _, server := range serverMap.data {
		for _, sub := range server.Subscriptions {
			if err := sub.Drain(); err != nil {
				slog.Error("drain subscription", "sub", sub.Subject, "error", err)
			}
		}
		server.EC.Close()
	}
}
