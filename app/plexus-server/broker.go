package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kr/pretty"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func broker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	slog.Info("Starting broker...")
	opts := &server.Options{}
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
	nc, err := nats.Connect("localhost:4222")
	if err != nil {
		slog.Error("nats connect", "error", err)
	}
	loginSub, err := nc.Subscribe("login.*", loginHandler)
	if err != nil {
		slog.Error("subscribe login", "error", err)
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
	//wg.Add(1)
	<-ctx.Done()
	loginSub.Drain()
	checkinSub.Drain()
	updateSub.Drain()
	configSub.Drain()
}

func loginHandler(msg *nats.Msg) {
	start := time.Now()
	name := msg.Subject[6:]
	slog.Info("login message", "name", name)
	pretty.Println("header", msg.Header)
	pretty.Println("repy", msg.Reply)
	pretty.Println("subject", msg.Subject)
	pretty.Println("data", string(msg.Data))
	pretty.Println("sub", msg.Sub.Queue, msg.Sub.Subject)
	msg.Respond([]byte("hello, " + name))
	slog.Info("login reply", "duration", time.Since(start))
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
