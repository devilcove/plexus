package agent

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"

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
	if !ns.ReadyForConnections(NatsTimeout) {
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
	ec.Subscribe("update", func(sub, reply string, data plexus.UpdateRequest) {
		response := plexus.NetworkResponse{}
		switch data.Action {
		case plexus.ConnectToNetwork:
			response = processConnect(data)
		case plexus.LeaveNetwork:
			response = processLeave(data)
		default:
			response.Error = true
			response.Message = "invalid request"
		}
		if err := ec.Publish(reply, response); err != nil {
			slog.Error("pub response to connect request", "error", err)
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
	for _, server := range self.Servers {
		ec, err := connectToServer(self, server)
		if err != nil {
			slog.Error("connect to server", "server", server, "error", err)
			continue
		}
		serverMap[server] = ec
	}
	for _, network := range networks {
		iface := network.Interface
		channel := make(chan bool, 1)
		ec, ok := serverMap[network.ServerURL]
		if !ok {
			slog.Error("network server not in list of servers", "network server", "network.ServerURL", "servers", self.Servers)
			continue
		}
		sub, err := ec.Subscribe("networks.>", networkUpdates)
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

func processLeave(request plexus.UpdateRequest) plexus.NetworkResponse {
	slog.Debug("leave", "network", request.Network)
	response := plexus.NetworkResponse{}
	errResponse := plexus.NetworkResponse{Error: true}
	network, err := boltdb.Get[plexus.Network](request.Network, "networks")
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	request.Peer.WGPublicKey = self.WGPublicKey
	conn, ok := serverMap[network.ServerURL]
	if !ok {
		slog.Debug(networkNotMapped)
		errResponse.Message = networkNotMapped
		return errResponse
	}
	if err := conn.Request("leave."+self.WGPublicKey, request, &response, NatsTimeout); err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	slog.Debug("leave complete")
	return response
}

func processConnect(request plexus.UpdateRequest) plexus.NetworkResponse {
	slog.Debug("connect", "network", request.Network, "server", request.Server)
	response := plexus.NetworkResponse{}
	errResponse := plexus.NetworkResponse{Error: true}
	_, err := boltdb.Get[plexus.Network](request.Network, "networks")
	if err == nil {
		slog.Debug(err.Error())
		errResponse.Message = "already connected to network"
		return errResponse
	}
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	serverEC, err := connectToServer(self, request.Server)
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	request.Peer = self.Peer
	if err := serverEC.Request("update."+self.WGPublicKey, request, &response, NatsTimeout); err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	serverEC.Close()
	addNewNetworks(self, response.Networks)
	connectToServers(self)
	return response
}

func addNewNetworks(self plexus.Device, networks []plexus.Network) {
	existingNetworks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("get existing networks", "error", err)
	}
	takenInterfaces := []int{}
	for _, existing := range existingNetworks {
		takenInterfaces = append(takenInterfaces, existing.InterfaceSuffix)
	}
	for _, network := range networks {
		network.ListenPort, err = getFreePort(defaultWGPort)
		if err != nil {
			slog.Debug(err.Error())
		}
		for i := range maxNetworks {
			if !slices.Contains(takenInterfaces, i) {
				network.InterfaceSuffix = i
				network.Interface = "plexus" + strconv.Itoa(i)
				takenInterfaces = append(takenInterfaces, i)
				break
			}
		}
		if err := boltdb.Save(network, network.Name, "networks"); err != nil {
			slog.Error("error saving network", "name", network.Name, "error", err)
		}
		if !slices.Contains(self.Servers, network.ServerURL) {
			self.Servers = append(self.Servers, network.ServerURL)
			if err := boltdb.Update(self, "self", "devices"); err != nil {
				slog.Error("update device with new server", "error", err)
			}
		}
		if err := startInterface(self, network); err != nil {
			slog.Error("start new interface", "error", err)
		}
	}
}
