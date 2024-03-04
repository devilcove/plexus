package agent

import (
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
	ec.Conn.Subscribe(">", func(msg *nats.Msg) {
		slog.Debug("received nats message", "subject", msg.Subject, "data", string(msg.Data))
	})
	ec.Subscribe("status", func(subject, reply string, data any) {
		slog.Debug("status request received")
		networks, err := boltdb.GetAll[plexus.Network](networkTable)
		if err != nil {
			slog.Error("get networks", "error", err)
		}
		self, err := boltdb.Get[plexus.Device]("self", deviceTable)
		if err != nil {
			slog.Error("get device", "error", err)
		}
		servers := []plexus.ServerConnection{}
		for _, serverName := range self.Servers {
			server := plexus.ServerConnection{Server: serverName}
			data, ok := serverMap[serverName]
			if !ok {
				server.Connected = "connection undefined"
				servers = append(servers, server)
				continue
			}
			if data.EC.Conn.IsConnected() {
				server.Connected = "connected"
			} else {
				server.Connected = "not connected"
			}
			servers = append(servers, server)
		}
		status := plexus.StatusResponse{
			Servers:    servers,
			Networks:   networks,
			ListenPort: self.PublicListenPort,
		}
		if err := ec.Publish(reply, status); err != nil {
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
		resp := reload(data)
		if err := ec.Publish(reply, resp); err != nil {
			slog.Error("pub reply to reload request", "error", err)
		}
		self, err := boltdb.Get[plexus.Device]("self", deviceTable)
		if err != nil {
			slog.Error("get device", "error", err)
			return
		}
		deleteAllNetworks()
		deleteAllInterfaces()
		addNewNetworks(self, resp.Networks)
	})
	ec.Subscribe("reset", func(sub, reply string, request *plexus.ResetRequest) {
		self, err := boltdb.Get[plexus.Device]("self", deviceTable)
		if err != nil {
			slog.Error(err.Error())
			if err := ec.Publish(reply, plexus.ServerResponse{Error: true, Message: err.Error()}); err != nil {
				slog.Error(err.Error())
			}
			return
		}
		network, err := boltdb.Get[plexus.Network](request.Network, networkTable)
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
		self, err := boltdb.Get[plexus.Device]("self", deviceTable)
		if err != nil {
			slog.Error("get device", "error", err)
			if err := ec.Publish(reply, response); err != nil {
				slog.Error("publish reply to version request", "error", err)
			}
			return
		}
		for server, data := range serverMap {
			slog.Debug("checking version of server", "server", server)
			if err := data.EC.Request("update."+self.WGPublicKey, plexus.AgentRequest{Action: plexus.Version, Args: strconv.FormatBool(*long)}, &serverResponse, NatsTimeout); err != nil {
				slog.Error("version request", "server", server, "error", err)
				continue
			}
			response.Servers = append(response.Servers, serverResponse.Version)
		}
		response.Agent = version + ": "
		if *long {
			info, _ := debug.ReadBuildInfo()
			for _, setting := range info.Settings {
				if strings.Contains(setting.Key, "vcs") {
					response.Agent = response.Agent + setting.Value + " "
				}
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

func connectToServers() {
	serverMap = make(map[string]serverData)
	networkMap = make(map[string]netMap)
	self, err := boltdb.Get[plexus.Device]("self", deviceTable)
	if err != nil {
		slog.Error("unable to read device", "error", err)
		return
	}
	for _, server := range self.Servers {
		slog.Info("connecting to server", "server", server)
		ec, err := connectToServer(self, server)
		if err != nil {
			slog.Error("connect to server", "server", server, "error", err)
			continue
		}
		serverData := serverData{EC: ec}
		networkUpdates, err := ec.Subscribe("networks.>", networkUpdates)
		if err != nil {
			slog.Error("network subscription failed", "error", err)
		}
		updates, err := ec.Subscribe(self.WGPublicKey, func(subject, reply string, data *plexus.DeviceUpdate) {
			switch data.Action {
			case plexus.LeaveServer:
				slog.Info("leave server", "server", data.Network.ServerURL)
				delete(serverMap, data.Network.ServerURL)
				ec.Close()
				deleteServer(data.Network.ServerURL)
			case plexus.JoinNetwork:
				slog.Info("join network", "network", data.Network, "server", data.Network.ServerURL)
				if err := connectToNetwork(data.Network); err != nil {
					slog.Error("connect to network", "error", err)
				}
			case plexus.Ping:
				slog.Debug("received ping from server", "server", server)
				if err := ec.Publish(reply, plexus.PingResponse{Message: "pong"}); err != nil {
					slog.Error("publish pong", "error", err)
				}
			default:
				slog.Error("invalid subject", "subj", data.Action)
			}
		})
		if err != nil {
			slog.Error("device subscription failed", "error", err)
		}
		serverData.Subscriptions = []*nats.Subscription{networkUpdates, updates}
		serverMap[server] = serverData

	}
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		slog.Error("unable to read networks", "error", err)
	}
	for _, network := range networks {
		iface := network.Interface
		_, ok := serverMap[network.ServerURL]
		if !ok {
			slog.Error("network server not in list of servers", "network server", network.ServerURL, "servers", self.Servers)
			continue
		}
		networkMap[network.Name] = netMap{
			Interface: iface,
		}
	}
	log.Println("server connection", serverMap, len(serverMap), networkMap)
}

func processLeave(request plexus.AgentRequest) plexus.ServerResponse {
	slog.Debug("leave", "network", request.Network)
	response := plexus.ServerResponse{}
	errResponse := plexus.ServerResponse{Error: true}
	network, err := boltdb.Get[plexus.Network](request.Network, networkTable)
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	self, err := boltdb.Get[plexus.Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	request.Peer.WGPublicKey = self.WGPublicKey
	server, ok := serverMap[network.ServerURL]
	if !ok {
		slog.Debug(networkNotMapped)
		errResponse.Message = networkNotMapped
		return errResponse
	}
	if err := server.EC.Request("leave."+self.WGPublicKey, request, &response, NatsTimeout); err != nil {
		slog.Debug(err.Error())
		errResponse.Message = err.Error()
		return errResponse
	}
	slog.Debug("leave complete")
	return response
}

func processJoin(request plexus.AgentRequest) plexus.ServerResponse {
	slog.Debug("join", "network", request.Network, "server", request.Server)
	response := plexus.ServerResponse{}
	errResponse := plexus.ServerResponse{Error: true}
	_, err := boltdb.Get[plexus.Network](request.Network, networkTable)
	if err == nil {
		slog.Warn("already connected to network")
		errResponse.Message = "already connected to network"
		return errResponse
	}
	self, err := boltdb.Get[plexus.Device]("self", deviceTable)
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
	connectToServers()
	return response
}

func connectToNetwork(network plexus.Network) error {
	self, err := boltdb.Get[plexus.Device]("self", deviceTable)
	if err != nil {
		return err
	}
	networks := []plexus.Network{network}
	addNewNetworks(self, networks)
	connectToServers()
	return nil
}

func addNewNetworks(self plexus.Device, networks []plexus.Network) {
	existingNetworks, err := boltdb.GetAll[plexus.Network](networkTable)
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
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("error saving network", "name", network.Name, "error", err)
		}
		if !slices.Contains(self.Servers, network.ServerURL) {
			slog.Debug("adding new server", "server", network.ServerURL)
			self.Servers = append(self.Servers, network.ServerURL)
			if err := boltdb.Save(self, "self", deviceTable); err != nil {
				slog.Error("update device with new server", "error", err)
			}
		} else {
			slog.Debug("server already exists in server list", "server", network.ServerURL, "list", self.Servers)
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

func reload(data *plexus.ReloadRequest) plexus.ServerResponse {
	response := plexus.ServerResponse{Error: true}
	serverResponse := plexus.ServerResponse{}
	self, err := boltdb.Get[plexus.Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
		response.Message = err.Error()
		return response
	}
	if data.Server != "" {
		server, ok := serverMap[data.Server]
		if !ok {
			response.Message = "invalid server"
			return response
		}
		if err := server.EC.Request("config."+self.WGPublicKey, nil, &serverResponse, NatsTimeout); err != nil {
			response.Message = "error from server" + err.Error()
			return response
		}
		return serverResponse
	}
	for name, server := range serverMap {
		if err := server.EC.Request("config."+self.WGPublicKey, nil, &serverResponse, NatsTimeout); err != nil {
			response.Message += "error from server" + name + err.Error()
			continue
		}
		response.Networks = append(response.Networks, serverResponse.Networks...)
	}
	if response.Message == "" {
		response.Error = false
		response.Message = "data reloaded successfully"
	}
	return response
}
