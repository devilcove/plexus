package agent

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
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

func refreshData(self plexus.Device) {
	slog.Info("getting data from servers")
	//delete existing networks
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Warn("get networks", "error", err)
	}
	for _, network := range networks {
		if err := boltdb.Delete[plexus.Network](network.Name, "networks"); err != nil {
			slog.Warn("delete network", "network", network.Name, "error", err)
		}
	}
	for key, ec := range serverMap {
		slog.Info("procssing server", "server", key)
		if ec == nil {
			slog.Error("nil nats connection", "key", key)
			continue
		}
		var networks []plexus.Network
		if err := ec.Request("config."+self.WGPublicKey, "helloworld", &networks, NatsTimeout); err != nil {
			slog.Error("refresh data", "server", key, "error", err)
			continue
		}
		slog.Info("refresh data", "msg", networks)
		for i, network := range networks {
			network.Interface = "plexus" + strconv.Itoa(i)
			network.ListenPort = self.ListenPort
			if err := boltdb.Save(network, network.Name, "networks"); err != nil {
				slog.Error("save network", "network", network.Name, "error", err)
			}
		}
	}
}

func setupSubs(ctx context.Context, wg *sync.WaitGroup, self plexus.Device) {
	networkMap = make(map[string]plexus.NetMap)
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("unable to read networks", "error", err)
	}
	for _, network := range networks {
		//start interface
		iface := network.Interface
		channel := make(chan bool, 1)
		networkMap[network.Name] = plexus.NetMap{
			Interface: iface,
			Channel:   channel,
		}
		if err := startInterface(self, network); err != nil {
			slog.Error("interface did not start", "name", iface, "network", network.Name, "error", err)
			return
		}
		wg.Add(1)
		go natSubscribe(ctx, wg, self, network, serverMap[network.ServerURL])
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
			slog.Info("nats error", "error", err)
		}),
		nats.Nkey(pk, sign),
	}...)
	nc, err := nats.Connect("nats://"+server+":4222", opts...)
	if err != nil {
		return nil, err
	}
	return nats.NewEncodedConn(nc, nats.JSON_ENCODER)
}

func natSubscribe(ctx context.Context, wg *sync.WaitGroup, self plexus.Device, network plexus.Network, ec *nats.EncodedConn) {
	defer wg.Done()
	sub, err := ec.Subscribe("networks."+network.Name, networkUpdates)
	if err != nil {
		slog.Error("network subcription failed", "error", err)
		return
	}
	<-ctx.Done()
	log.Println("mq shutting down")
	if err := sub.Drain(); err != nil {
		slog.Error("drain subscriptions", "error", err)
	}
	ec.Close()
	slog.Info("networks subs exititing", "network", network.Name)
}

func checkin(wg *sync.WaitGroup) {
	defer wg.Done()
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("get device", "error", err)
		return
	}
	for server, ec := range serverMap {
		if !ec.Conn.IsConnected() {
			slog.Debug("not connected to server broker .... skipping checkin", "server", server)
			continue
		}
		msg, err := ec.Conn.Request("checkin."+self.WGPublicKey, []byte("checking"), NatsTimeout)
		if err != nil {
			slog.Error("error publishing checkin ", "error", err)
			continue
		}
		log.Println("checkin response from server", server, string(msg.Data))
	}
	publishConnectivity(self)
}

func closeServerConnections() {
	for _, ec := range serverMap {
		for _, network := range networkMap {
			if err := network.Subscription.Drain(); err != nil {
				slog.Error("drain subscription", "sub", network.Subscription.Subject, "error", err)
			}
		}
		ec.Close()
	}
}
