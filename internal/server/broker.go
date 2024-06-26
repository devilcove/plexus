package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

func broker(ctx context.Context, wg *sync.WaitGroup, tls *tls.Config) {
	defer wg.Done()
	slog.Info("Starting broker...")
	//create admin user
	adminKey := getAdminKey()
	adminPublicKey, err := adminKey.PublicKey()
	if err != nil {
		slog.Error("could not create admin public key", "error", err)
		brokerfail <- 1
		return
	}
	tokensUsers := getTokenUsers()
	deviceUsers := getDeviceUsers()
	natsOptions = &server.Options{
		Nkeys: []*server.NkeyUser{
			{
				Nkey: adminPublicKey,
				Permissions: &server.Permissions{
					Publish: &server.SubjectPermission{
						Allow: []string{">"},
					},
					Subscribe: &server.SubjectPermission{
						Allow: []string{">"},
					},
				},
			},
		},
	}
	natsOptions.Nkeys = append(natsOptions.Nkeys, tokensUsers...)
	natsOptions.Nkeys = append(natsOptions.Nkeys, deviceUsers...)
	natsOptions.NoSigs = true
	if config.Secure {
		natsOptions.TLSConfig = tls
		natsOptions.Host = config.FQDN
	}
	natServer, err = server.NewServer(natsOptions)
	if err != nil {
		slog.Error("nats server", "error", err)
		return
	}
	go natServer.Start()
	if !natServer.ReadyForConnections(natsTimeout) {
		slog.Error("not ready for connection", "error", err)
		return
	}
	SignatureCB := func(nonce []byte) ([]byte, error) {
		return adminKey.Sign(nonce)
	}
	opts := []nats.Option{nats.Nkey(adminPublicKey, SignatureCB)}
	natsConn, err = nats.Connect(fmt.Sprintf("nats://%s:4222", config.FQDN), opts...)
	if err != nil {
		slog.Error("nats connect", "error", err)
		brokerfail <- 1
	}
	eConn, err = nats.NewEncodedConn(natsConn, nats.JSON_ENCODER)
	if err != nil {
		slog.Error("nats encoded connect", "error", err)
		brokerfail <- 1
	}

	subscrptions := serverSubcriptions()

	slog.Info("broker started")
	pingTicker := time.NewTicker(pingTick)
	keyTicker := time.NewTicker(keyTick)
	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down broker")
			pingTicker.Stop()
			keyTicker.Stop()
			for _, sub := range subscrptions {
				_ = sub.Drain()
			}
			return
		case token := <-newDevice:
			slog.Info("new login device", "device", token)
			keyValue, err := plexus.DecodeToken(token)
			if err != nil {
				slog.Error("decode token", "error", err)
			}
			key, err := nkeys.FromSeed([]byte(keyValue.Seed))
			if err != nil {
				slog.Error("seed failure", "error", err)
				continue
			}
			nPubKey, err := key.PublicKey()
			if err != nil {
				slog.Error("publickey", "error", err)
				continue
			}
			natsOptions.Nkeys = append(natsOptions.Nkeys, &server.NkeyUser{
				Nkey:        nPubKey,
				Permissions: registerPermissions(),
			})
			_ = natServer.ReloadOptions(natsOptions)
		case <-pingTicker.C:
			pingPeers()
		case <-keyTicker.C:
			expireKeys()
		}
	}
}

func getTokenUsers() []*server.NkeyUser {
	users := []*server.NkeyUser{}
	keys, err := boltdb.GetAll[plexus.Key](keyTable)
	if err != nil {
		slog.Error("unable to retrieve keys", "error", err)
	}
	for _, key := range keys {
		token, err := plexus.DecodeToken(key.Value)
		if err != nil {
			slog.Error("decodetoken", "error", err)
			continue
		}
		users = append(users, createNkeyUser(token.Seed))
	}
	return users
}

func createNkeyUser(token string) *server.NkeyUser {
	kp, err := nkeys.FromSeed([]byte(token))
	if err != nil {
		slog.Error("unable to create keypair", "error", err)
		return nil
	}
	pk, err := kp.PublicKey()
	if err != nil {
		slog.Error("unable to create public key", "error", err)
	}
	return &server.NkeyUser{
		Nkey:        pk,
		Permissions: registerPermissions(),
	}
}

func getAdminKey() nkeys.KeyPair {
	seed, err := os.ReadFile(path + "server.seed")
	if err != nil {
		return createAdminNKeyPair()
	}
	kp, err := nkeys.FromSeed(seed)
	if err != nil {
		return createAdminNKeyPair()
	}
	return kp
}

func createAdminNKeyPair() nkeys.KeyPair {
	admin, err := nkeys.CreateUser()
	if err != nil {
		slog.Error("could not create admin nkey pair", "error", err)
		brokerfail <- 1
		return admin
	}
	seed, err := admin.Seed()
	if err != nil {
		slog.Error("admin seed creation", "error", err)
		return admin
	}
	if err := os.WriteFile(path+"server.seed", seed, os.ModePerm); err != nil {
		slog.Error("save admin seed", "error", err)
	}
	return admin
}

func devicePermissions(id string) *server.Permissions {
	return &server.Permissions{
		Publish: &server.SubjectPermission{
			Allow: []string{
				id + ".>",
			},
		},
		Subscribe: &server.SubjectPermission{
			Allow: []string{"networks.>", plexus.Update + id + ".>", "_INBOX.>"},
		},
		Response: &server.ResponsePermission{
			MaxMsgs: 1,
		},
	}
}

func registerPermissions() *server.Permissions {
	return &server.Permissions{
		Publish: &server.SubjectPermission{
			Allow: []string{"register"},
		},
		Subscribe: &server.SubjectPermission{
			Allow: []string{"_INBOX.>"},
		},
	}
}

func serverSubcriptions() []*nats.Subscription {
	subcriptions := []*nats.Subscription{}

	//token subscriptions
	// register handler
	register, err := eConn.Subscribe("register", func(subj, reply string, request *plexus.ServerRegisterRequest) {
		response := registerHandler(request)
		slog.Debug("publish register reply", "response", response)
		if err := eConn.Publish(reply, response); err != nil {
			slog.Error("publish register reply", "error", err)
		}
		if err := decrementKeyUsage(request.KeyName); err != nil {
			slog.Error("decrement key usage", "error", err)
		}
	})
	if err != nil {
		slog.Error("subscribe register", "error", err)
	}
	subcriptions = append(subcriptions, register)

	//device subscriptions
	//checkin
	checkin, err := eConn.Subscribe("*"+plexus.Checkin, func(subj, reply string, request *plexus.CheckinData) {
		if len(subj) != 52 {
			slog.Error("invalid subj", "subj", subj)
			if err := eConn.Publish(reply, plexus.ErrorResponse{Message: "invalid subject"}); err != nil {
				slog.Error("publish error", "error", err)
			}
			return
		}
		if err := eConn.Publish(reply, processCheckin(request)); err != nil {
			slog.Error("publish reply", "error", err)
		}
	})
	if err != nil {
		slog.Error("subcribe checkin", "error", err)
	}
	subcriptions = append(subcriptions, checkin)

	//join
	join, err := eConn.Subscribe("*"+plexus.JoinNetwork, func(subj, reply string, request *plexus.JoinRequest) {
		if len(subj) != 49 {
			slog.Error("invalid subj", "subj", subj)
			_ = eConn.Publish(reply, plexus.ErrorResponse{Message: "invalid subject"})
			return
		}
		if err := eConn.Publish(reply, processJoin(subj[:44], request)); err != nil {
			slog.Error("publish reply", "error", err)
		}
	})
	if err != nil {
		slog.Error("subcribe join", "error", err)
	}
	subcriptions = append(subcriptions, join)

	//version
	version, err := eConn.Subscribe("*"+plexus.Version, func(subj, reply string, request *any) {
		if len(subj) != 52 {
			slog.Error("invalid subj", "subj", subj)
			if err := eConn.Publish(reply, plexus.ErrorResponse{Message: "invalid subject"}); err != nil {
				slog.Error("publish error response", "error", err)
			}
			return
		}
		if err := eConn.Publish(reply, serverVersion()); err != nil {
			slog.Error("publish reply", "error", err)
		}
	})
	if err != nil {
		slog.Error("subcribe version", "error", err)
	}
	subcriptions = append(subcriptions, version)

	//leave
	leave, err := eConn.Subscribe("*"+plexus.LeaveNetwork, func(subj, reply string, request *plexus.LeaveRequest) {
		if len(subj) != 57 {
			slog.Error("invalid subj", "subj", subj)
			if err := eConn.Publish(reply, plexus.ErrorResponse{Message: "invalid subject"}); err != nil {
				slog.Error("publish error response", "error", err)
			}
			return
		}
		if err := eConn.Publish(reply, processLeave(subj[:44], request)); err != nil {
			slog.Error("publish reply", "error", err)
		}
	})
	if err != nil {
		slog.Error("subcribe leave", "error", err)
	}
	subcriptions = append(subcriptions, leave)

	//leaveServer
	leaveServer, err := eConn.Subscribe("*"+plexus.LeaveServer, func(subj, reply string, request *any) {
		if len(subj) != 56 {
			slog.Error("invalid subj", "subj", subj)
			if err := eConn.Publish(reply, plexus.ErrorResponse{Message: "invalid subject"}); err != nil {
				slog.Error("publish error response", "error", err)
			}
			return
		}
		if err := processLeaveServer(subj[:44]); err != nil {
			slog.Error("leave server", "error", err)
		}
	})
	if err != nil {
		slog.Error("subcribe leave server", "error", err)
	}
	subcriptions = append(subcriptions, leaveServer)

	//reload
	reload, err := eConn.Subscribe("*"+plexus.Reload, func(subj, reply string, request *any) {
		if len(subj) != 51 {
			slog.Error("invalid subj", "subj", subj)
			if err := eConn.Publish(reply, plexus.ErrorResponse{Message: "invalid subject"}); err != nil {
				slog.Error("publish error response", "error", err)
			}
			return
		}
		if err := eConn.Publish(reply, processReload(subj[:44])); err != nil {
			slog.Error("publish reply", "error", err)
		}
	})
	if err != nil {
		slog.Error("subcribe reload", "error", err)
	}
	subcriptions = append(subcriptions, reload)

	//listenPortUpdate
	portUpdate, err := eConn.Subscribe("*"+plexus.UpdateListenPorts, func(subj string, request *plexus.ListenPortResponse) {
		if len(subj) != 44+len(plexus.UpdateListenPorts) {
			slog.Error("invalid sub", "subj", subj)
			return
		}
		processPortUpdate(subj[:44], request)
	})
	if err != nil {
		slog.Error("subscribe network peer update", "error", err)
	}
	subcriptions = append(subcriptions, portUpdate)

	//deviceUpdate
	deviceUpdate, err := eConn.Subscribe("*"+plexus.UpdatePeer, func(subj string, request *plexus.Peer) {
		if len(subj) != 44+len(plexus.UpdatePeer) {
			slog.Error("invalid sub", "subj", subj)
			return
		}
		processDeviceUpdate(subj[:44], request)
	})
	if err != nil {
		slog.Error("subscribe device updates", "error", err)
	}
	subcriptions = append(subcriptions, deviceUpdate)

	// network peer updates
	peerUpdate, err := eConn.Subscribe("*"+plexus.UpdateNetworkPeer, func(subj string, request *plexus.NetworkPeer) {
		if len(subj) != 44+len(plexus.UpdateNetworkPeer) {
			slog.Error("invalid sub", "subj", subj)
			return
		}
		processNetworkPeerUpdate(subj[:44], request)
	})
	if err != nil {
		slog.Error("subscribe peer update", "error", err)
	}
	subcriptions = append(subcriptions, peerUpdate)

	return subcriptions
}
