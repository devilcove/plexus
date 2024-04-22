package agent

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

func startBroker() (*server.Server, *nats.EncodedConn) {
	defer log.Println("Agent server halting")
	ns, err := server.NewServer(&server.Options{Host: "localhost", Port: Config.NatsPort, NoSigs: true})
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
	_, _ = ec.Subscribe(Agent+plexus.Status, func(subject, reply string, data any) {
		slog.Debug("status request received")
		if err := ec.Publish(reply, processStatus()); err != nil {
			slog.Error("publish status response", "err", err)
		}
	})
	_, _ = ec.Subscribe(Agent+plexus.JoinNetwork, func(subj, reply string, data *plexus.JoinRequest) {
		slog.Debug("join request")
		if err := ec.Publish(reply, processJoin(data)); err != nil {
			slog.Error("publish join response", "error", err)
		}
	})
	_, _ = ec.Subscribe(Agent+plexus.LeaveNetwork, func(subj, reply string, data *plexus.LeaveRequest) {
		slog.Debug("leave request")
		if err := ec.Publish(reply, processLeave(data)); err != nil {
			slog.Error("publish leave response", "error", err)
		}
	})
	_, _ = ec.Subscribe(Agent+plexus.LeaveServer, func(subj, reply string, data *any) {
		slog.Debug("leaveServer request")
		if err := ec.Publish(reply, processLeaveServer()); err != nil {
			slog.Error("publish leaveServer response", "error", err)
		}
		if err := ec.Publish(reply, plexus.MessageResponse{Message: "disconnected from server"}); err != nil {
			slog.Error("publish reply", "error", err)
		}
	})

	_, _ = ec.Subscribe(Agent+plexus.Register, func(sub, reply string, data *plexus.RegisterRequest) {
		slog.Debug("register request")
		resp := handleRegistration(data)
		if err := ec.Publish(reply, resp); err != nil {
			slog.Error("publish reply", "error", err)
		}
	})
	_, _ = ec.Subscribe(Agent+plexus.LogLevel, func(level *plexus.LevelRequest) {
		slog.Debug("loglevel request")
		newLevel := strings.ToUpper(level.Level)
		slog.Info("loglevel change", "level", newLevel)
		plexus.SetLogging(newLevel)

	})
	_, _ = ec.Subscribe(Agent+plexus.Reload, func(sub, reply string, data *any) {
		slog.Debug("reload request")
		resp, err := processReload()
		if err != nil {
			if err := ec.Publish(reply, plexus.MessageResponse{Message: "error" + err.Error()}); err != nil {
				slog.Error("publish reply", "error", err)
			}
			return
		}
		self, err := boltdb.Get[Device]("self", deviceTable)
		if err != nil {
			slog.Error("get device", "error", err)
			if err := ec.Publish(reply, plexus.MessageResponse{Message: "error" + err.Error()}); err != nil {
				slog.Error("pub reply to reload request", "error", err)
			}
			return
		}
		if err := ec.Publish(reply, resp); err != nil {
			slog.Error("pub reply to reload request", "error", err)
		}
		deleteAllNetworks()
		deleteAllInterfaces()
		if err := saveServerNetworks(self, resp.Networks); err != nil {
			slog.Error("save networks", "error", err)
		}
		startAllInterfaces(self)
		//addNewNetworks(self, resp.Networks)
	})
	_, _ = ec.Subscribe(Agent+plexus.Reset, func(sub, reply string, request *plexus.ResetRequest) {
		slog.Debug("reset request")
		self, err := boltdb.Get[Device]("self", deviceTable)
		if err != nil {
			slog.Error(err.Error())
			if err := ec.Publish(reply, plexus.MessageResponse{Message: err.Error()}); err != nil {
				slog.Error(err.Error())
			}
			return
		}
		network, err := boltdb.Get[Network](request.Network, networkTable)
		if errors.Is(err, boltdb.ErrNoResults) {
			if err := ec.Publish(reply, plexus.MessageResponse{Message: "no such network"}); err != nil {
				slog.Error(err.Error())
			}
			return
		}
		if err != nil {
			slog.Error(err.Error())
			if err := ec.Publish(reply, plexus.MessageResponse{Message: err.Error()}); err != nil {
				slog.Error(err.Error())
			}
			return
		}
		if err := deleteInterface(network.Interface); err != nil {
			slog.Error("delete interface", "iface", network.Interface, "error", err)
		}
		if err := startInterface(self, network); err != nil {
			slog.Error("start interface", "iface", network.Interface, "error", err)
		}
		//if err := resetPeersOnNetworkInterface(self, network); err != nil {
		//	slog.Error(err.Error())
		//	if err := ec.Publish(reply, plexus.MessageResponse{Message: err.Error()}); err != nil {
		//		slog.Error(err.Error())
		//	}
		//	return
		//}
		if err := ec.Publish(reply, plexus.MessageResponse{Message: "interface reset"}); err != nil {
			slog.Error(err.Error())
		}
	})
	_, _ = ec.Subscribe(Agent+plexus.Version, func(sub, reply string, long *bool) {
		slog.Debug("version request")
		response := plexus.VersionResponse{}
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
			if err := serverEC.Request(self.WGPublicKey+plexus.Version, nil, &response, NatsTimeout); err != nil {
				slog.Error("version request", "error", err)
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
		if err := ec.Publish(reply, response); err != nil {
			slog.Error("publish reply to version request", "error", err)
		}
	})
}

func ConnectToAgentBroker() (*nats.EncodedConn, error) {
	url := fmt.Sprintf("nats://localhost:%d", Config.NatsPort)
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
	id := self.WGPublicKey
	serverEC := serverConn.Load()
	networkUpdates, err := serverEC.Subscribe("networks.>", networkUpdates)
	if err != nil {
		slog.Error("network subscription failed", "error", err)
	}
	subscriptions = append(subscriptions, networkUpdates)

	ping, err := serverEC.Subscribe(plexus.Update+id+plexus.Ping, func(subj, reply string, data *any) {
		if err := serverEC.Publish(reply, plexus.PingResponse{Message: "pong"}); err != nil {
			slog.Error("publish pong", "error", err)
		}
	})
	if err != nil {
		slog.Error("ping subscription", "error", err)
	}
	subscriptions = append(subscriptions, ping)

	leaveServer, err := serverEC.Subscribe(plexus.Update+id+plexus.LeaveServer, func(subj, reply string, data *any) {
		slog.Info("leave server")
		closeServerConnections()
		deleteAllInterfaces()
		deleteAllNetworks()
	})
	if err != nil {
		slog.Error("leave server subscription", "error", err)
	}
	subscriptions = append(subscriptions, leaveServer)

	joinNet, err := serverEC.Subscribe(plexus.Update+id+plexus.JoinNetwork, func(data *plexus.ServerJoinRequest) {
		slog.Info("join network", "network", data.Network)
		network, err := saveServerNetwork(data.Network)
		if err != nil {
			slog.Error("save network", "error", err)
			return
		}
		if err := startInterface(self, network); err != nil {
			slog.Error("error starting interface", "interface", network.Interface, "network", network.Name, "error", err)
			return
		}
	})
	if err != nil {
		slog.Error("join network subscription", "error", err)
	}
	subscriptions = append(subscriptions, joinNet)
	sendListenPorts, err := serverEC.Subscribe(plexus.Update+id+plexus.SendListenPorts,
		func(subj, reply string, data plexus.ListenPortRequest) {
			slog.Info("new listen ports", "network", data.Network)
			response, err := getNewListenPorts(data.Network)
			if err != nil {
				slog.Error(err.Error())
				return
			}
			if err := serverEC.Publish(reply, response); err != nil {
				slog.Error("publish reply to SendListenPorts", "error", err)
			}
			slog.Debug("sent listenports to server", "public", response.PublicListenPort, "private", response.ListenPort)
		})
	if err != nil {
		slog.Error("send listen port subscription", "error", err)
	}
	subscriptions = append(subscriptions, sendListenPorts)
	addRouter, err := serverEC.Subscribe(plexus.Update+id+plexus.AddRouter,
		func(subj, reply string, data plexus.NetworkPeer) {
			if data.WGPublicKey != id {
				slog.Error("add router wrong id", "me", id, "router", data.WGPublicKey)
				return
			}
			if !data.IsSubnetRouter {
				return
			}
			slog.Debug("adding subnet router")
			if data.UseNat {
				if err := addNat(); err != nil {
					slog.Error("add nat", "error", err)
				}
			}
			if data.UseVirtSubnet {
				if err := addVirtualSubnet(data.VirtSubnet, data.Subnet); err != nil {
					slog.Error("add virtual subnet", "error", err)
				}
			}
		})
	if err != nil {
		slog.Error("add router subscription", "error", err)
	}
	subscriptions = append(subscriptions, addRouter)
	delRouter, err := serverEC.Subscribe(plexus.Update+id+plexus.DeleteRouter,
		func(subj, reply string, data plexus.NetworkPeer) {
			if data.WGPublicKey != id {
				slog.Error("add router wrong id", "me", id, "router", data.WGPublicKey)
				return
			}
			if err := delNat(); err != nil {
				slog.Error("delete nat", "error", err)
			}
			if err := delVirtualSubnet(); err != nil {
				slog.Error("delete virtual subnet", "error", err)
			}
		})
	if err != nil {
		slog.Error("delete router subscription", "error", err)
	}
	subscriptions = append(subscriptions, delRouter)
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
