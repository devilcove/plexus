package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
)

func socketServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	os.Remove("/tmp/unixsock")
	listener, err := net.Listen("unix", "/tmp/unixsock")
	if err != nil {
		slog.Error("opening socket, ...exitiing", "error", err)
		return
	}
	slog.Info("listening on /tmp/unixsock")
	go func(listener net.Listener, ctx context.Context) {
		<-ctx.Done()
		err := listener.Close()
		if err != nil {
			slog.Error("close socket", "error", err)
		}
	}(listener, ctx)
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("unable to accept", "error", err)
			break
		}
		slog.Info("client connected")
		wg.Add(1)
		go handleConnection(ctx, wg, conn)
	}
}

func handleConnection(ctx context.Context, wg *sync.WaitGroup, conn net.Conn) {
	defer wg.Done()
	defer func() {
		err := conn.Close()
		if err != nil {
			slog.Error("close connection", "error", err)
		}
	}()
	go func(ctx context.Context) {
		<-ctx.Done()
		if err := conn.Close(); err != nil {
			slog.Error("close connection", "error", err)
		}
	}(ctx)

	buffer := make([]byte, 1024)
	for {
		count, err := conn.Read(buffer)
		if err != nil {
			if errors.Is(err, io.EOF) {
				slog.Info("connection closed")
				return
			}
			slog.Error("read connection", "error", err)
			return
		}
		data := buffer[0:count]

		command := plexus.Command{}
		if err := json.Unmarshal(data, &command); err != nil {
			slog.Error("unmarshal command", "data", string(data), "error", err)
			return
		}
		pretty.Println(command)
		switch command.Command {
		case "join":
			if err := processJoin(command); err != nil {
				slog.Error("process join", "error", err)
			}
			conn.Write([]byte("join"))
		case "status":
			slog.Debug("status request")
			status, err := getStatus()
			if err != nil {
				slog.Error("getStatus", "error", err)
				conn.Write([]byte("status error " + err.Error()))
				return
			}
			payload, err := json.Marshal(status)
			if err != nil {
				slog.Error("marshal status", "error", err)
				conn.Write([]byte("status error " + err.Error()))
				return
			}
			conn.Write(payload)
		default:
			slog.Warn("unknow command", "command", command.Command)
		}
	}
}
