package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func startAgentNatsServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ns, err := server.NewServer(&server.Options{Host: "localhost", Port: 4223})
	if err != nil {
		slog.Error("start nats", "error", err)
		panic(err)
	}
	go ns.Start()
	defer ns.Shutdown()
	if !ns.ReadyForConnections(3 * time.Second) {
		slog.Error("nats not ready for connections")
		panic("not ready for connections")
	}
	slog.Info("nats server started")
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		slog.Error("nats connect", "error", err)
		natsfail <- struct{}{}
		return
	}
	defer nc.Close()
	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		slog.Error("nats encoder", "error", err)
		natsfail <- struct{}{}
		return
	}
	nc.Subscribe(">", func(msg *nats.Msg) {
		fmt.Println("recieved msg on ", msg.Subject, "topic ", string(msg.Data))
	})
	ec.Subscribe("status", func(subject, reply string, data any) {
		slog.Debug("status request received")
		networks, err := boltdb.GetAll[plexus.Network]("networks")
		if err != nil {
			slog.Error("get networks", "error", err)
		}
		if err := ec.Publish(reply, networks); err != nil {
			slog.Error("status response", "error", err)
		}
	})
	ec.Subscribe("leave", func(subject, reply string, data plexus.LeaveRequest) {
		response := plexus.LeaveResponse{}
		var err error
		response.Message, err = processLeave(data.Network)
		if err != nil {
			response.Error = true
			response.Message = err.Error()
		}
		if err := ec.Publish(reply, response); err != nil {
			slog.Error("pub response to leave request", "error", err)
		}
	})
	defer ec.Close()
	generalCh := make(chan *nats.Msg, 64)
	if _, err := nc.ChanSubscribe(">", generalCh); err != nil {
		slog.Error("channel sub", "error", err)
	}
	ackCh := make(chan *string)
	ec.BindSendChan("ack", ackCh)
	joinCh := make(chan *plexus.JoinCommand)
	ec.BindRecvChan("join", joinCh)
	loglevelCh := make(chan *plexus.LevelRequest)
	if _, err := ec.BindRecvChan("loglevel", loglevelCh); err != nil {
		slog.Error("bind channel", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-generalCh:
			slog.Debug("recieved", "msg", msg)
		case request := <-joinCh:
			slog.Debug("join request received", "request", request)
			if err := processJoin(request); err != nil {
				slog.Error("join", "error", err)
			}
		case level := <-loglevelCh:
			newLevel := strings.ToUpper(level.Level)
			slog.Info("loglevel change", "level", newLevel)
			plexus.SetLogging(newLevel)
		}
	}

}

func ConnectToAgentBroker() (*nats.EncodedConn, error) {
	url := "nats://localhost:4223"
	slog.Debug("connecting to agent broker ", "url", url)
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return nil, err
	}
	return ec, nil
}

func connectToServers(self plexus.Device) {
	serverMap = make(map[string]*nats.EncodedConn)
	networkMap = make(map[string]plexus.NetMap)
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("unable to read networks", "error", err)
	}
	for _, network := range networks {
		ec, err := connectToServer(self, network.ServerURL)
		if err != nil {
			slog.Error("unable to connect to server", "server", network.ServerURL, "error", err)
		}
		serverMap[network.ServerURL] = ec
		iface := network.Interface
		channel := make(chan bool, 1)
		if err := startInterface(self, network); err != nil {
			slog.Error("start interface", "name", iface, "network", network, "error", err)
		}
		sub, err := ec.Subscribe("networks."+network.Name, networkUpdates)
		if err != nil {
			slog.Error("network subscription failed", "error", err)
		}
		networkMap[network.Name] = plexus.NetMap{
			Interface:    iface,
			Channel:      channel,
			Subscription: sub,
		}
	}
	fmt.Println("servermap", serverMap, "length", len(serverMap))
	fmt.Println("networkmap", networkMap)
}

func processLeave(net string) (string, error) {
	slog.Debug("leave", "network", net)
	network, err := boltdb.Get[plexus.Network](net, "networks")
	if err != nil {
		return "", err
	}
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		return "", err
	}
	conn, ok := serverMap[network.ServerURL]
	if !ok {
		return "", errors.New("network not mapped to server")
	}
	msg, err := conn.Conn.Request("leave."+self.WGPublicKey, []byte(network.Name), time.Second*5)
	if err != nil {
		return "", err
	}
	slog.Debug("leave complete")
	return string(msg.Data), nil
}
