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
	admin, err := nkeys.CreateUser()
	if err != nil {
		slog.Error("could not create admin user", "error", err)
		brokerfail <- 1
		return
	}
	adminPublicKey, err := admin.PublicKey()
	if err != nil {
		slog.Error("could not create admin public key", "error", err)
		brokerfail <- 1
		return
	}
	seed, err := admin.Seed()
	if err == nil {
		if err := os.WriteFile("/tmp/seed", seed, os.ModePerm); err != nil {
			slog.Error("could not save seed", "error", err)
		}
	} else {
		slog.Error("seed", "error", err)
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
		//Users: users
	}
	natsOptions.Nkeys = append(natsOptions.Nkeys, tokensUsers...)
	natsOptions.Nkeys = append(natsOptions.Nkeys, deviceUsers...)
	natServer, err = server.NewServer(natsOptions)
	if err != nil {
		slog.Error("nats server", "error", err)
		return
	}
	go natServer.Start()
	if !natServer.ReadyForConnections(3 * time.Second) {
		slog.Error("not ready for connection", "error", err)
		return
	}
	sign := func(nonce []byte) ([]byte, error) {
		return admin.Sign(nonce)
	}
	connectOpts := nats.Options{
		Url:         "nats://localhost:4222",
		Nkey:        adminPublicKey,
		Name:        "nats-test-nkey",
		SignatureCB: sign,
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
	// join handler
	joinSub, err := encodedConn.Subscribe("join", func(subj, reply string, request *plexus.JoinRequest) {
		response := joinHandler(request)
		slog.Debug("publish join reply", "response", response)
		if err := encodedConn.Publish(reply, response); err != nil {
			slog.Error("join", "error", err)
		}
	})
	if err != nil {
		slog.Error("subscribe join", "error", err)
	}
	checkinSub, err := natsConn.Subscribe("checkin.*", checkinHandler)
	if err != nil {
		slog.Error("subscribe checkin", "error", err)
	}
	updateSub, err := encodedConn.Subscribe("update.*", func(subj, reply string, request *plexus.NetworkRequest) {
		response := processUpdate(request)
		slog.Debug("pubish update rely", "respone", response)
		if err := encodedConn.Publish(reply, response); err != nil {
			slog.Error("update", "error", err)
		}
	})
	if err != nil {
		slog.Error("subscribe update", "error", err)
	}
	configSub, err := natsConn.Subscribe("config.*", configHandler)
	if err != nil {
		slog.Error("subscribe config", "error", err)
	}
	connectivitySub, err := natsConn.Subscribe("connectivity.*", connectivityHandler)
	if err != nil {
		slog.Error("subscribe connectivity", "error", err)
	}
	leaveSub, err := natsConn.Subscribe("leave.*", leaveHandler)
	if err != nil {
		slog.Error("subscribe leave", "error", err)
	}

	slog.Info("broker started")
	//wg.Add(1)
	for {
		select {
		case <-ctx.Done():
			joinSub.Drain()
			checkinSub.Drain()
			updateSub.Drain()
			configSub.Drain()
			connectivitySub.Drain()
			leaveSub.Drain()
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
				Nkey: nPubKey,
				Permissions: &server.Permissions{
					Publish: &server.SubjectPermission{
						Allow: []string{"join"},
					},
					Subscribe: &server.SubjectPermission{
						Allow: []string{"_INBOX.>"},
					},
				},
			})
			natServer.ReloadOptions(natsOptions)
		}
	}
}

func getTokenUsers() []*server.NkeyUser {
	users := []*server.NkeyUser{}
	keys, err := boltdb.GetAll[plexus.Key]("keys")
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
		Nkey: pk,
		Permissions: &server.Permissions{
			Publish: &server.SubjectPermission{
				Allow: []string{"join"},
			},
			Subscribe: &server.SubjectPermission{
				Allow: []string{"_INBOX.>"},
			},
		},
	}
}
