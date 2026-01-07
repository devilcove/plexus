package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net"
	"runtime/debug"
	"strings"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/publish"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

func startBroker() (*server.Server, *nats.Conn) {
	defer log.Println("Agent server halting")
	ns, err := server.NewServer(
		&server.Options{Host: "localhost", Port: Config.NatsPort, NoSigs: true},
	)
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
	subcribe(nc)
	return ns, nc
}

func subcribe(agentConn *nats.Conn) {
	_, _ = agentConn.Subscribe(Agent+plexus.Status, func(msg *nats.Msg) {
		if err := agentConn.Publish(msg.Reply, processStatus()); err != nil {
			slog.Error("publish status response", "error", err)
		}
	})

	_, _ = agentConn.Subscribe(Agent+plexus.JoinNetwork, func(msg *nats.Msg) {
		slog.Debug("join request")
		if err := agentConn.Publish(msg.Reply, serviceJoin(msg.Data)); err != nil {
			slog.Error("publish join response", "error", err)
		}
	})
	_, _ = agentConn.Subscribe(Agent+plexus.LeaveNetwork, func(msg *nats.Msg) {
		slog.Debug("leave request")
		if err := agentConn.Publish(msg.Reply, handleLeave(msg.Data)); err != nil {
			slog.Error("publish leave response", "error", err)
		}
	})
	_, _ = agentConn.Subscribe(Agent+plexus.LeaveServer, func(msg *nats.Msg) {
		slog.Debug("leaveServer request")
		resp, err := handleLeaveServer()
		if err != nil {
			slog.Error("invalid leave server response", "error", err)
		}
		if err := agentConn.Publish(msg.Reply, resp); err != nil {
			slog.Error("publish leave network response", "error", err)
		}
	})

	_, _ = agentConn.Subscribe(Agent+plexus.Register, func(msg *nats.Msg) {
		slog.Debug("register request")
		resp := processRegistration(msg.Data)
		if err := agentConn.Publish(msg.Reply, resp); err != nil {
			slog.Error("publish reply", "error", err)
		}
	})
	_, _ = agentConn.Subscribe(Agent+plexus.LogLevel, func(msg *nats.Msg) {
		slog.Debug("loglevel request")
		level := &plexus.LevelRequest{}
		if err := json.Unmarshal(msg.Data, level); err != nil {
			slog.Error("invalid log level request", "error", err, "data", string(msg.Data))
			return
		}
		newLevel := strings.ToUpper(level.Level)
		slog.Info("loglevel change", "level", newLevel)
		plexus.SetLogging(newLevel)
	})
	_, _ = agentConn.Subscribe(Agent+plexus.Reload, func(msg *nats.Msg) {
		sendRelaad(msg, agentConn)
	})
	_, _ = agentConn.Subscribe(Agent+plexus.Reset, func(msg *nats.Msg) {
		sendReset(msg, agentConn)
	})
	_, _ = agentConn.Subscribe(Agent+plexus.Version, func(msg *nats.Msg) {
		sendVersion(msg, agentConn)
	})
	_, _ = agentConn.Subscribe(Agent+plexus.SetPrivateEndpoint, func(msg *nats.Msg) {
		setPrivateEndpoint(msg, agentConn)
	})
}

func ConnectToAgentBroker() (*nats.Conn, error) {
	url := fmt.Sprintf("nats://localhost:%d", Config.NatsPort)
	slog.Debug("connecting to agent broker ", "url", url)
	agentConn, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return agentConn, nil
}

func subcribeToServerTopics(self Device) {
	id := self.WGPublicKey
	serverConn := serverConn.Load()
	networkUpdates, err := serverConn.Subscribe("networks.>", networkUpdates)
	if err != nil {
		slog.Error("network subscription failed", "error", err)
	}
	subscriptions = append(subscriptions, networkUpdates)

	ping, err := serverConn.Subscribe(plexus.Update+id+plexus.Ping, func(msg *nats.Msg) {
		publish.Message(serverConn, msg.Reply, plexus.PingResponse{Message: "pong"})
	})
	if err != nil {
		slog.Error("ping subscription", "error", err)
	}
	subscriptions = append(subscriptions, ping)

	leaveServer, err := serverConn.Subscribe(
		plexus.Update+id+plexus.LeaveServer,
		func(_ *nats.Msg) {
			slog.Info("leave server")
			closeServerConnections()
			deleteAllInterfaces()
			deleteAllNetworks()
		},
	)
	if err != nil {
		slog.Error("leave server subscription", "error", err)
	}
	subscriptions = append(subscriptions, leaveServer)

	joinNet, err := serverConn.Subscribe(plexus.Update+id+plexus.JoinNetwork, func(msg *nats.Msg) {
		joinNetwork(msg, self)
	})
	if err != nil {
		slog.Error("join network subscription", "error", err)
	}
	subscriptions = append(subscriptions, joinNet)
	sendListenPorts, err := serverConn.Subscribe(plexus.Update+id+plexus.SendListenPorts,
		func(msg *nats.Msg) {
			sendListenPorts(msg, serverConn)
		})
	if err != nil {
		slog.Error("send listen port subscription", "error", err)
	}
	subscriptions = append(subscriptions, sendListenPorts)
	addRouter, err := serverConn.Subscribe(plexus.Update+id+plexus.AddRouter,
		func(msg *nats.Msg) {
			addRouter(msg, id)
		})
	if err != nil {
		slog.Error("add router subscription", "error", err)
	}
	subscriptions = append(subscriptions, addRouter)
	delRouter, err := serverConn.Subscribe(plexus.Update+id+plexus.DeleteRouter,
		func(msg *nats.Msg) {
			deleteRouter(msg, id)
		})
	if err != nil {
		slog.Error("delete router subscription", "error", err)
	}
	subscriptions = append(subscriptions, delRouter)
}

func createRegistationConnection(key plexus.KeyValue) (*nats.Conn, error) {
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
	return opts.Connect()
}

func Request(conn *nats.Conn, subj string, request any, response any, timeout time.Duration) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	msg, err := conn.Request(subj, data, timeout)
	if err != nil {
		return err
	}
	return json.Unmarshal(msg.Data, response)
}

func setPrivateEndpoint(msg *nats.Msg, agentConn *nats.Conn) {
	request := plexus.PrivateEndpoint{}
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		slog.Error("invalid private endpoint request", "error", err, "data", string(msg.Data))
		publish.ErrorMessage(agentConn, msg.Reply, "invalid request", err)
		return
	}
	slog.Debug("set private endpoint", "endpoint", request.IP, "network", request.Network)
	var err error
	var networks []Network
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		publish.ErrorMessage(agentConn, msg.Reply, "get device", err)
		return
	}
	if request.Network == "" {
		networks, err = boltdb.GetAll[Network](networkTable)
		if err != nil {
			publish.ErrorMessage(agentConn, msg.Reply, "get network", err)
			return
		}
	} else {
		network, err := boltdb.Get[Network](request.Network, networkTable)
		if err != nil {
			publish.ErrorMessage(agentConn, msg.Reply, "get network", err)
		}
		networks = append(networks, network)
	}
	for _, network := range networks {
		for i, peer := range network.Peers {
			if peer.WGPublicKey == self.WGPublicKey {
				network.Peers[i].PrivateEndpoint = net.ParseIP(request.IP)
				network.Peers[i].UsePrivateEndpoint = false
				if err := publishNetworkPeerUpdate(self, &network.Peers[i]); err != nil {
					publish.ErrorMessage(agentConn, msg.Reply, "publish error", err)
				}
			}
		}
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			publish.ErrorMessage(agentConn, msg.Reply, "internal error", err)
		}
	}
	restartEndpointServer <- struct{}{}
	// wait to ensure endpoint server is started.
	time.Sleep(time.Millisecond * 10)
	publish.Message(agentConn, msg.Reply, "private endpoint added")
}

func sendVersion(msg *nats.Msg, agentConn *nats.Conn) {
	slog.Debug("version request")
	response := plexus.VersionResponse{}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		publish.ErrorMessage(agentConn, msg.Reply, "get self", err)
		slog.Error("get device", "error", err)
		return
	}
	slog.Debug("checking version of server")
	server := serverConn.Load()
	if server != nil {
		slog.Debug("server connection", "connected", server.IsConnected())
		resp, err := server.Request(self.WGPublicKey+plexus.Version, nil, NatsTimeout)
		if err != nil {
			slog.Error("version request", "error", err)
		}
		if err := json.Unmarshal(resp.Data, &response); err != nil {
			slog.Error(
				"invalid version response from server",
				"error", err,
				"data", string(resp.Data),
			)
		}
	} else {
		slog.Debug("not connected to server")
	}
	response.Agent = version + ": "
	info, _ := debug.ReadBuildInfo()
	for _, setting := range info.Settings {
		if strings.Contains(setting.Key, "vcs") {
			response.Agent = response.Agent + setting.Value + " "
		}
	}
	bytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("invalid version response", "error", err, "data", response)
	}
	if err := agentConn.Publish(msg.Reply, bytes); err != nil {
		slog.Error("publish reply to version request", "error", err)
	}
}

func sendReset(msg *nats.Msg, agentConn *nats.Conn) {
	slog.Debug("reset request")
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error(err.Error())
		publish.ErrorMessage(agentConn, msg.Reply, "get device", err)
		return
	}
	request := &plexus.ResetRequest{}
	if err := json.Unmarshal(msg.Data, request); err != nil {
		slog.Error("invalid reset request", "error", err, "data", string(msg.Data))
		publish.ErrorMessage(agentConn, msg.Reply, "invalid request", err)
		return
	}
	network, err := boltdb.Get[Network](request.Network, networkTable)
	if err != nil {
		publish.ErrorMessage(agentConn, msg.Reply, "get network", err)
		return
	}
	if err := deleteInterface(network.Interface); err != nil {
		slog.Error("delete interface", "iface", network.Interface, "error", err)
	}
	if err := startInterface(self, network); err != nil {
		slog.Error("start interface", "iface", network.Interface, "error", err)
	}
	publish.Message(agentConn, msg.Reply, "interfaces reset")
}

func sendRelaad(msg *nats.Msg, agentConn *nats.Conn) {
	slog.Debug("reload request")
	resp, err := processReload()
	if err != nil {
		publish.ErrorMessage(agentConn, msg.Reply, "process reload", err)
		return
	}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
		publish.ErrorMessage(agentConn, msg.Reply, "get device", err)
		return
	}
	bytes, err := json.Marshal(resp)
	if err != nil {
		slog.Error("invalid reload response", "error", err, "data", resp)
	} else {
		if err := agentConn.Publish(msg.Reply, bytes); err != nil {
			slog.Error("pub reply to reload request", "error", err)
		}
	}
	deleteAllNetworks()
	deleteAllInterfaces()
	if err := saveServerNetworks(self, resp.Networks); err != nil {
		slog.Error("save networks", "error", err)
	}
	startAllInterfaces(self)
	// addNewNetworks(self, resp.Networks).
}
