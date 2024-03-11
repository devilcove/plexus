package main

import (
	"context"
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

func broker(ctx context.Context, wg *sync.WaitGroup) {
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
	//TODO :: add users
	// users := GetUsers()
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
	connectOpts := nats.Options{
		Url:  "nats://localhost:4222",
		Nkey: adminPublicKey,
		Name: "nats-test-nkey",
		SignatureCB: func(nonce []byte) ([]byte, error) {
			return adminKey.Sign(nonce)
		},
	}
	natsConn, err = connectOpts.Connect()
	if err != nil {
		slog.Error("nats connect", "error", err)
		brokerfail <- 1
	}
	encodedConn, err = nats.NewEncodedConn(natsConn, nats.JSON_ENCODER)
	if err != nil {
		slog.Error("nats encoded connect", "error", err)
		brokerfail <- 1
	}

	// register handler
	registerSub, err := encodedConn.Subscribe("register", func(subj, reply string, request *plexus.ServerRegisterRequest) {
		response := registerHandler(request)
		slog.Debug("publish register reply", "response", response)
		if err := encodedConn.Publish(reply, response); err != nil {
			slog.Error("publish register reply", "error", err)
		}
		if err := decrementKeyUsage(request.KeyName); err != nil {
			slog.Error("decrement key usage", "error", err)
		}
	})
	if err != nil {
		slog.Error("subscribe register", "error", err)
	}
	//checkinSub, err := encodedConn.Subscribe("checkin.*", func(subj, reply string, request *plexus.CheckinData) {
	//	slog.Debug("checkin", "peer", request.ID)
	//	response := processCheckin(request)
	//	if err := encodedConn.Publish(reply, response); err != nil {
	//		slog.Error("publish checkin response", err)
	//	}
	//})
	//if err != nil {
	//	slog.Error("subscribe checkin", "error", err)
	//}
	updateSub, err := encodedConn.Subscribe("update.*", func(subj, reply string, request *plexus.AgentRequest) {
		peer := subj[7:]
		if request.Peer.WGPublicKey != "" && request.Peer.WGPublicKey != peer {
			slog.Error("invalid update requests", "subject", peer, "peer", request.Peer.WGPublicKey, "networkPeer", request.NetworkPeer.WGPublicKey)
			return
		}
		if request.NetworkPeer.WGPublicKey != "" && request.NetworkPeer.WGPublicKey != peer {
			slog.Error("invalid update requests", "subject", peer, "peer", request.Peer.WGPublicKey, "networkPeer", request.NetworkPeer.WGPublicKey)
			return
		}
		slog.Debug("update request from", "peer", peer, "request", request.Action)
		response := processUpdate(request)
		if request.Action == plexus.GetConfig {
			response = configHandler(subj)
		}
		slog.Debug("publish update rely", "respone", response)
		if err := encodedConn.Publish(reply, response); err != nil {
			slog.Error("update", "error", err)
		}
	})
	if err != nil {
		slog.Error("subscribe update", "error", err)
	}
	//configSub, err := encodedConn.Subscribe("config.*", func(sub, reply string, request any) {
	//response := configHandler(sub)
	//	if err := encodedConn.Publish(reply, response); err != nil {
	//		slog.Error("pub response to config request", "error", err)
	//	}
	//})
	//if err != nil {
	//	slog.Error("subcribe config", "error", err)
	//}
	//leaveSub, err := encodedConn.Subscribe("leave.*", func(subj, reply string, request *plexus.AgentRequest) {
	//	response := processLeave(request)
	//	slog.Debug("publish leave reply", "response", response)
	//	if err := encodedConn.Publish(reply, response); err != nil {
	//		slog.Error("leave reply", "error", err)
	//	}
	//})
	//if err != nil {
	//	slog.Error("subscribe leave", "error", err)
	//}

	slog.Info("broker started")
	pingTicker := time.NewTicker(pingTick)
	keyTicker := time.NewTicker(keyTick)
	for {
		select {
		case <-ctx.Done():
			pingTicker.Stop()
			keyTicker.Stop()
			registerSub.Drain()
			//checkinSub.Drain()
			updateSub.Drain()
			//configSub.Drain()
			//leaveSub.Drain()
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
			natServer.ReloadOptions(natsOptions)
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
