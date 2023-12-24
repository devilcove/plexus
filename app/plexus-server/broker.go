package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

func broker(ctx context.Context, wg *sync.WaitGroup, c chan int) {
	defer wg.Done()
	slog.Info("Starting broker...")
	//create admin user
	admin, err := nkeys.CreateUser()
	if err != nil {
		slog.Error("could not create admin user", "error", err)
		c <- 1
		return
	}
	adminPublicKey, err := admin.PublicKey()
	if err != nil {
		slog.Error("could not create admin public key", "error", err)
		c <- 1
		return
	}
	//TODO :: add users
	// users := GetUsers()
	opts := &server.Options{
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
	ns, err := server.NewServer(opts)
	if err != nil {
		slog.Error("nats server", "error", err)
		return
	}
	go ns.Start()
	if !ns.ReadyForConnections(3 * time.Second) {
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
	nc, err := connectOpts.Connect()
	if err != nil {
		slog.Error("nats connect", "error", err)
		c <- 1
	}
	loginSub, err := nc.Subscribe("join", joinHandler)
	if err != nil {
		slog.Error("subscribe join", "error", err)
	}
	checkinSub, err := nc.Subscribe("checkin.*", checkinHandler)
	if err != nil {
		slog.Error("subscribe checkin", "error", err)
	}
	updateSub, err := nc.Subscribe("update.*", updateHandler)
	if err != nil {
		slog.Error("subscribe update", "error", err)
	}
	configSub, err := nc.Subscribe("config.*", configHandler)
	if err != nil {
		slog.Error("subscribe config", "error", err)
	}
	slog.Info("broker started")
	//wg.Add(1)
	<-ctx.Done()
	loginSub.Drain()
	checkinSub.Drain()
	updateSub.Drain()
	configSub.Drain()
}

func joinHandler(msg *nats.Msg) {
	slog.Info("join handler")
	response := "hello " + string(msg.Data)
	msg.Respond([]byte(response))
}

func checkinHandler(m *nats.Msg) {
	device := m.Subject[7:]
	//update, err := database.GetDevice(device)
	slog.Info("received checkin", "device", device)
	m.Respond([]byte("ack"))
}

func updateHandler(m *nats.Msg) {
	device := m.Subject[7:]
	//update, err := database.GetDevice(device)
	slog.Info("received update", "device", device, "update", string(m.Data))
	m.Respond([]byte("update ack"))
}

func configHandler(m *nats.Msg) {
	device := m.Subject[7:]
	slog.Info("received config request", "device", device)
	m.Respond([]byte("config ack"))
}

func getPubNkey(u string) (string, error) {
	user, err := boltdb.Get[plexus.Peer](u, "peers")
	if err != nil {
		return "", err
	}
	return user.PubNkey, nil
}
