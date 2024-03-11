package agent

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

func startAgentNatsServer() (*server.Server, *nats.EncodedConn) {
	defer log.Println("Agent server halting")
	ns, err := server.NewServer(&server.Options{Host: "localhost", Port: 4223, NoSigs: true})
	if err != nil {
		slog.Error("start nats", "error", err)
		panic(err)
	}
	ns.Start()
	if !ns.ReadyForConnections(NatsTimeout) {
		slog.Error("nats not ready for connections")
		panic("not ready for connections")
	}
	slog.Info("nats server started")
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		slog.Error("nats connect", "error", err)
		return nil, nil
	}
	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		slog.Error("nats encoder", "error", err)
		return nil, nil
	}
	subcribe(ec)
	return ns, ec
}

func subcribe(ec *nats.EncodedConn) {
	ec.Subscribe(">", func(subj string, msg *any) {
		slog.Debug("received nats message", "subject", subj, "data", *msg)
	})
	ec.Subscribe("status", func(subject, reply string, data any) {
		slog.Debug("status request received")
		networks, err := boltdb.GetAll[Network](networkTable)
		if err != nil {
			slog.Error("get networks", "error", err)
		}
		response := StatusResponse{Networks: networks}
		self, err := boltdb.Get[Device]("self", deviceTable)
		if err != nil {
			slog.Error("get device", "error", err)
		}
		response.Server = self.Server
		if ec.Conn.IsConnected() {
			response.Connected = "connected"
		} else {
			response.Connected = "not connected"
		}
		if err := ec.Publish(reply, response); err != nil {
			slog.Error("status response", "error", err)
		}
	})
	ec.Subscribe("update", func(sub, reply string, data plexus.AgentRequest) {
		response := plexus.ServerResponse{}
		switch data.Action {
		case plexus.JoinNetwork:
			response = processJoin(data)
		case plexus.LeaveNetwork:
			response = processLeave(data)
		case plexus.LeaveServer:
			response = processLeaveServer(data)
		default:
			response.Error = true
			response.Message = "invalid request"
		}
		if err := ec.Publish(reply, response); err != nil {
			slog.Error("pub response to connect request", "error", err)
		}
	})
	ec.Subscribe("register", func(sub, reply string, data *plexus.RegisterRequest) {
		resp := registerPeer(data)
		ec.Publish(reply, resp)
	})
	ec.Subscribe("loglevel", func(level *plexus.LevelRequest) {
		newLevel := strings.ToUpper(level.Level)
		slog.Info("loglevel change", "level", newLevel)
		plexus.SetLogging(newLevel)

	})
	ec.Subscribe("reload", func(sub, reply string, data *plexus.ReloadRequest) {
		resp := reload()
		if err := ec.Publish(reply, resp); err != nil {
			slog.Error("pub reply to reload request", "error", err)
		}
		self, err := boltdb.Get[Device]("self", deviceTable)
		if err != nil {
			slog.Error("get device", "error", err)
			return
		}
		deleteAllNetworks()
		deleteAllInterfaces()
		addNewNetworks(self, resp.Networks)
	})
	ec.Subscribe("reset", func(sub, reply string, request *plexus.ResetRequest) {
		self, err := boltdb.Get[Device]("self", deviceTable)
		if err != nil {
			slog.Error(err.Error())
			if err := ec.Publish(reply, plexus.ServerResponse{Error: true, Message: err.Error()}); err != nil {
				slog.Error(err.Error())
			}
			return
		}
		network, err := boltdb.Get[Network](request.Network, networkTable)
		if errors.Is(err, boltdb.ErrNoResults) {
			if err := ec.Publish(reply, plexus.ServerResponse{Error: true, Message: "no such network"}); err != nil {
				slog.Error(err.Error())
			}
			return
		}
		if err != nil {
			slog.Error(err.Error())
			if err := ec.Publish(reply, plexus.ServerResponse{Error: true, Message: err.Error()}); err != nil {
				slog.Error(err.Error())
			}
			return
		}
		if err := resetPeersOnNetworkInterface(self, network); err != nil {
			slog.Error(err.Error())
			if err := ec.Publish(reply, plexus.ServerResponse{Error: true, Message: err.Error()}); err != nil {
				slog.Error(err.Error())
			}
			return
		}
		if err := ec.Publish(reply, plexus.ServerResponse{Message: "interface reset"}); err != nil {
			slog.Error(err.Error())
		}
	})
	ec.Subscribe("version", func(sub, reply string, long *bool) {
		slog.Debug("version request")
		response := plexus.VersionResponse{}
		serverResponse := plexus.ServerResponse{}
		self, err := boltdb.Get[Device]("self", deviceTable)
		if err != nil {
			slog.Error("get device", "error", err)
			if err := ec.Publish(reply, response); err != nil {
				slog.Error("publish reply to version request", "error", err)
			}
			return
		}
		slog.Debug("checking version of server")
		serverEC := serverConn.Load()
		if serverEC != nil {
			slog.Debug("server connection", "connected", serverEC.Conn.IsConnected())
			if err := serverEC.Request("update."+self.WGPublicKey, plexus.AgentRequest{
				Action: plexus.Version}, &serverResponse, NatsTimeout); err != nil {
				slog.Error("version request", "error", err)
			}
		} else {
			slog.Debug("not connected to server")
		}
		response.Server = serverResponse.Version
		response.Agent = version + ": "
		info, _ := debug.ReadBuildInfo()
		for _, setting := range info.Settings {
			if strings.Contains(setting.Key, "vcs") {
				response.Agent = response.Agent + setting.Value + " "
			}
		}
		if err := ec.Publish(reply, response); err != nil {
			slog.Error("publish reply to version request", "error", err)
		}
	})
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

func subcribeToServerTopics(self Device) {
	serverEC := serverConn.Load()
	networkUpdates, err := serverEC.Subscribe("networks.>", networkUpdates)
	if err != nil {
		slog.Error("network subscription failed", "error", err)
	}
	updates, err := serverEC.Subscribe(self.WGPublicKey, func(subject, reply string, data *plexus.DeviceUpdate) {
		switch data.Action {
		case plexus.LeaveServer:
			slog.Info("leave server", "server", data.Server)
			closeServerConnections()
			deleteAllInterfaces()
			deleteAllNetworks()
		case plexus.JoinNetwork:
			slog.Info("join network", "network", data.Network, "server", data.Server)
			if err := connectToNetwork(data.Network); err != nil {
				slog.Error("connect to network", "error", err)
			}
		case plexus.SendListenPorts:
			slog.Info("server requested listen ports")
			response, err := getNewListenPorts(data.Network)
			if err != nil {
				slog.Error(err.Error())
				return
			}
			if err := serverEC.Publish(reply, response); err != nil {
				slog.Error("publish reply to SendListenPorts", "error", err)
			}
			slog.Debug("sent listenports to server", "public", response.PublicListenPort, "private", response.ListenPort)
		case plexus.Ping:
			slog.Debug("received ping from server")
			if err := serverEC.Publish(reply, plexus.PingResponse{Message: "pong"}); err != nil {
				slog.Error("publish pong", "error", err)
			}
		default:
			slog.Error("invalid subject", "subj", data.Action)
		}
	})
	if err != nil {
		slog.Error("device subscription failed", "error", err)
	}
	subscriptions = []*nats.Subscription{networkUpdates, updates}
}

func processLeave(request plexus.AgentRequest) plexus.ServerResponse {
	slog.Debug("leave", "network", request.Network)
	response := plexus.ServerResponse{}
	errResponse := plexus.ServerResponse{Error: true}
	network, err := boltdb.Get[Network](request.Network, networkTable)
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = fmt.Sprintf("%v, %v", err, errors.Is(err, boltdb.ErrExists))
		return errResponse
	}
	if err := deleteInterface(network.Interface); err != nil {
		slog.Debug("delete interface", "error", err)
		errResponse.Message = "failed to delete interface: " + err.Error()
		return errResponse
	}
	if err := boltdb.Delete[Network](request.Network, networkTable); err != nil {
		slog.Debug(err.Error())
		errResponse.Message = fmt.Sprintf("%v, %v", err, errors.Is(err, boltdb.ErrExists))
		return errResponse
	}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	request.Peer.WGPublicKey = self.WGPublicKey
	serverEC := serverConn.Load()
	if err := serverEC.Request("leave."+self.WGPublicKey, request, &response, NatsTimeout); err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	slog.Debug("leave complete")
	return response
}

func processLeaveServer(request plexus.AgentRequest) plexus.ServerResponse {
	slog.Debug("leave", "server", request.Server)
	response := plexus.ServerResponse{}
	errResponse := plexus.ServerResponse{Error: true}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	serverEC := serverConn.Load()
	if err := serverEC.Request(self.WGPublicKey, request, &response, NatsTimeout); err != nil {
		errResponse.Message = "error publishing request to server: " + err.Error()
		return errResponse
	}
	self.Server = ""
	if err := boltdb.Save(self, "self", deviceTable); err != nil {
		slog.Error("save device", "error", err)
	}
	return response
}

func processJoin(request plexus.AgentRequest) plexus.ServerResponse {
	slog.Debug("join", "network", request.Network)
	response := plexus.ServerResponse{}
	errResponse := plexus.ServerResponse{Error: true}
	_, err := boltdb.Get[Network](request.Network, networkTable)
	if err == nil {
		slog.Warn("already connected to network")
		errResponse.Message = "already connected to network"
		return errResponse
	}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	request.Peer = self.Peer
	slog.Debug("sending join request to server")
	serverEC := serverConn.Load()
	if err := serverEC.Request("update."+self.WGPublicKey, request, &response, NatsTimeout); err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	slog.Debug("adding network", "networks", response.Networks)
	if len(response.Networks) == 0 {
		slog.Debug("empty network response from server")
		errResponse.Message = "no network obtained from server"
		return errResponse
	}
	addNewNetworks(self, response.Networks)
	return response
}

func connectToNetwork(network plexus.Network) error {
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		return err
	}
	networks := []plexus.Network{network}
	addNewNetworks(self, networks)
	//connectToServers()
	return nil
}

func addNewNetworks(self Device, serverNets []plexus.Network) {
	existingNetworks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		slog.Error("get existing networks", "error", err)
	}
	takenInterfaces := []int{}
	for _, existing := range existingNetworks {
		takenInterfaces = append(takenInterfaces, existing.InterfaceSuffix)
	}
	slog.Debug("taken interfaces", "taken", takenInterfaces)
	for _, serverNet := range serverNets {
		network := toAgentNetwork(serverNet)
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
		slog.Debug("saving network", "network", network.Name)
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("error saving network", "name", network.Name, "error", err)
		}
		if err := startInterface(self, network); err != nil {
			slog.Error("start new interface", "error", err)
		}
	}
}

func createRegistationConnection(key plexus.KeyValue) (*nats.EncodedConn, error) {
	loginKeyPair, err := nkeys.FromSeed([]byte(key.Seed))
	if err != nil {
		return nil, err
	}
	loginPublicKey, err := loginKeyPair.PublicKey()
	if err != nil {
		return nil, err
	}
	sign := func(nonce []byte) ([]byte, error) {
		return loginKeyPair.Sign(nonce)
	}
	opts := nats.Options{
		Url:         "nats://" + key.URL + ":4222",
		Nkey:        loginPublicKey,
		SignatureCB: sign,
	}
	nc, err := opts.Connect()
	if err != nil {
		return nil, err
	}
	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return nil, err
	}
	return ec, nil
}

func reload() plexus.ServerResponse {
	response := plexus.ServerResponse{Error: true}
	serverResponse := plexus.ServerResponse{}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
		response.Message = err.Error()
		return response
	}
	serverEC := serverConn.Load()
	if err := serverEC.Request("config."+self.WGPublicKey, nil, &serverResponse, NatsTimeout); err != nil {
		response.Message = "error from server" + err.Error()
		return response
	}
	return serverResponse
}
