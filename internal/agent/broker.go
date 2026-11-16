package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"strings"
	"time"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/publish"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

func startBroker() (*server.Server, *nats.Conn) {
	defer slog.Info("Agent server halting")
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
	agentConn, err = nats.Connect(ns.ClientURL())
	if err != nil {
		slog.Error("nats connect", "error", err)
		return nil, nil
	}
	subcribe()
	return ns, agentConn
}

func subcribe() {
	for _, sub := range []struct {
		subject  string
		callback nats.MsgHandler
	}{
		{Agent + plexus.Status, subscribeStatus},
		{Agent + plexus.JoinNetwork, subscribeJoinNetwork},
		{Agent + plexus.LeaveNetwork, subscribeLeaveNetwork},
		{Agent + plexus.LeaveServer, subscribeLeaveServer},
		{Agent + plexus.Register, subscribeRegister},
		{Agent + plexus.LogLevel, subscribeLogLevel},
		{Agent + plexus.Reload, sendReload},
		{Agent + plexus.Reset, sendReset},
		{Agent + plexus.Version, sendVersion},
		{Agent + plexus.SetPrivateEndpoint, setPrivateEndpoint},
	} {
		_, _ = agentConn.Subscribe(sub.subject, sub.callback)
	}
}

func subscribeStatus(msg *nats.Msg) {
	if err := agentConn.Publish(msg.Reply, processStatus()); err != nil {
		slog.Error("publish status response", "error", err)
	}
}

func subscribeJoinNetwork(msg *nats.Msg) {
	slog.Debug("join request")
	if err := agentConn.Publish(msg.Reply, serviceJoin(msg.Data)); err != nil {
		slog.Error("publish join response", "error", err)
	}
}

func subscribeLeaveNetwork(msg *nats.Msg) {
	slog.Debug("leave request")
	if err := agentConn.Publish(msg.Reply, handleLeave(msg.Data)); err != nil {
		slog.Error("publish leave response", "error", err)
	}
}

func subscribeLeaveServer(msg *nats.Msg) {
	slog.Debug("leaveServer request")
	resp, err := handleLeaveServer()
	if err != nil {
		slog.Error("invalid leave server response", "error", err)
	}
	if err := agentConn.Publish(msg.Reply, resp); err != nil {
		slog.Error("publish leave network response", "error", err)
	}
}

func subscribeRegister(msg *nats.Msg) {
	slog.Debug("register request")
	resp := processRegistration(msg.Data)
	if err := agentConn.Publish(msg.Reply, resp); err != nil {
		slog.Error("publish reply", "error", err)
	}
}

func subscribeLogLevel(msg *nats.Msg) {
	slog.Debug("loglevel request")
	level := &plexus.LevelRequest{}
	if err := json.Unmarshal(msg.Data, level); err != nil {
		slog.Error("invalid log level request", "error", err, "data", string(msg.Data))
		return
	}
	newLevel := strings.ToUpper(level.Level)
	slog.Info("loglevel change", "level", newLevel)
	plexus.SetLogging(newLevel)
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
	subs := []struct {
		subject  string
		callback nats.MsgHandler
	}{
		{"networks.>", networkUpdates},
		{plexus.Update + id + plexus.Ping, func(msg *nats.Msg) {
			publish.Message(serverConn, msg.Reply, plexus.PingResponse{Message: "pong"})
		}},
		{plexus.Update + id + plexus.LeaveServer, func(_ *nats.Msg) {
			slog.Info("leave server")
			closeServerConnections()
			deleteAllInterfaces()
			deleteAllNetworks()
		}},
		{plexus.Update + id + plexus.JoinNetwork, func(msg *nats.Msg) {
			joinNetwork(msg, self)
		}},
		{plexus.Update + id + plexus.SendListenPorts, func(msg *nats.Msg) {
			sendListenPorts(msg, serverConn)
		}},
		{plexus.Update + id + plexus.AddRouter, func(msg *nats.Msg) {
			addRouter(msg, id)
		}},
		{plexus.Update + id + plexus.DeleteRouter, func(msg *nats.Msg) {
			deleteRouter(msg, id)
		}},
	}

	for _, s := range subs {
		sub, err := serverConn.Subscribe(s.subject, s.callback)
		if err != nil {
			slog.Error("subscription error", "subject", s.subject, "error", err)
			continue
		}
		subscriptions = append(subscriptions, sub)
	}
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

func setPrivateEndpoint(msg *nats.Msg) {
	request := plexus.PrivateEndpoint{}
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		slog.Error("invalid private endpoint request", "error", err, "data", string(msg.Data))
		publish.ErrorMessage(agentConn, msg.Reply, "invalid request", err)
		return
	}
	slog.Debug("set private endpoint", "endpoint", request.IP, "network", request.Network)
	var err error
	var networks []Network
	self, err := store.Get[Device]("self", deviceTable)
	if err != nil {
		publish.ErrorMessage(agentConn, msg.Reply, "get device", err)
		return
	}
	if request.Network == "" {
		networks, err = store.GetAll[Network](networkTable)
		if err != nil {
			publish.ErrorMessage(agentConn, msg.Reply, "get network", err)
			return
		}
	} else {
		network, err := store.Get[Network](request.Network, networkTable)
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
		if err := store.Save(network, network.Name, networkTable); err != nil {
			publish.ErrorMessage(agentConn, msg.Reply, "internal error", err)
		}
	}
	restartEndpointServer <- struct{}{}
	// wait to ensure endpoint server is started.
	time.Sleep(time.Millisecond * 10)
	publish.Message(agentConn, msg.Reply, plexus.MessageResponse{Message: "private endpoint added"})
}

func sendVersion(msg *nats.Msg) {
	slog.Debug("version request")
	response := plexus.VersionResponse{}
	self, err := store.Get[Device]("self", deviceTable)
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
	info, _ := debug.ReadBuildInfo()
	response.Agent += info.Main.Version
	bytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("invalid version response", "error", err, "data", response)
	}
	if err := agentConn.Publish(msg.Reply, bytes); err != nil {
		slog.Error("publish reply to version request", "error", err)
	}
}

func sendReset(msg *nats.Msg) {
	slog.Debug("reset request")
	self, err := store.Get[Device]("self", deviceTable)
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
	network, err := store.Get[Network](request.Network, networkTable)
	if err != nil {
		slog.Error("get network", "network", request.Network, "error", err)
		publish.ErrorMessage(agentConn, msg.Reply, "get network", err)
		return
	}
	if err := deleteInterface(network.Interface); err != nil {
		slog.Error("delete interface", "iface", network.Interface, "error", err)
		publish.ErrorMessage(agentConn, msg.Reply, "delete interface "+network.Interface, err)
		return
	}
	if err := startInterface(self, network); err != nil {
		slog.Error("start interface", "iface", network.Interface, "error", err)
		publish.ErrorMessage(agentConn, msg.Reply, "start interface "+network.Interface, err)
		return
	}
	publish.Message(agentConn, msg.Reply, plexus.MessageResponse{Message: "interfaces reset"})
}

func sendReload(msg *nats.Msg) {
	slog.Debug("reload request")
	resp, err := processReload()
	if err != nil {
		publish.ErrorMessage(agentConn, msg.Reply, "process reload", err)
		return
	}
	self, err := store.Get[Device]("self", deviceTable)
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
