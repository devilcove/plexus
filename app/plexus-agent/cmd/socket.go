package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
)

var socket string

func socketServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	socket = os.Getenv("HOME") + "/.local/share/plexus/socket"
	os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		slog.Error("opening socket, ...exitiing", "error", err)
		return
	}
	slog.Info("listening on ", "socket", socket)
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
		//if err := conn.SetDeadline(time.Now().Add(time.Second * 10)); err != nil {
		//	slog.Error("set deadline", "error", err)
		//	break
		//}

		slog.Info("client connected")
		wg.Add(1)
		go handleConnection(ctx, wg, conn)
	}
}

func handleConnection(ctx context.Context, wg *sync.WaitGroup, conn net.Conn) {
	defer wg.Done()
	defer conn.Close()
	go func(ctx context.Context) {
		<-ctx.Done()
		if err := conn.Close(); err != nil {
			slog.Error("close connection", "error", err)
		}
	}(ctx)

	buffer := make([]byte, 1024)
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
		response := "join successful"
		if err := processJoin(command); err != nil {
			slog.Error("process join", "error", err)
			response = err.Error()
		}
		payload, err := json.Marshal(response)
		if err != nil {
			slog.Error("marshal", "error", err)
			return
		}
		conn.Write(payload)
	case "status":
		slog.Debug("status request")
		networks, err := getStatus()
		slog.Debug("status", "network count", len(networks))
		if err != nil {
			slog.Error("getStatus", "error", err)
			conn.Write([]byte("status error " + err.Error()))
			return
		}
		payload, err := json.Marshal(networks)
		if err != nil {
			slog.Error("marshal status", "error", err)
			conn.Write([]byte("status error " + err.Error()))
			return
		}
		slog.Debug("writing response to socket")
		if _, err := conn.Write(payload); err != nil {
			slog.Error("write to socket", "error", err)
			return
		}
	case "leave":
		response := fmt.Sprintf("left network %s", command.Data)
		if err := leaveNetwork(command.Data.(string)); err != nil {
			slog.Error("leave network", "error", err)
			response = err.Error()
		}
		payload, err := json.Marshal(response)
		if err != nil {
			slog.Error("marshal", "error", err)
			return
		}
		if _, err := conn.Write(payload); err != nil {
			slog.Error("socket write", "error", err)
			return
		}
	default:
		slog.Warn("unknow command", "command", command.Command)
	}
}

func sendToDaemon[T any](msg plexus.Command) (T, error) {
	var resp T
	socket := os.Getenv("HOME") + "/.local/share/plexus/socket"
	c, err := net.DialTimeout("unix", socket, time.Second*10)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("unable to connect to agent daemon, is daemon running?")
	}
	if err != nil {
		slog.Debug("net dial", "error", err)
		return resp, err
	}
	defer c.Close()
	payload, err := json.Marshal(msg)
	if err != nil {
		slog.Debug("marshal plexus.Command", "error", err)
		return resp, err
	}
	if _, err := c.Write(payload); err != nil {
		slog.Debug("write payload", "error", err)
		return resp, err
	}
	data, err := io.ReadAll(c)
	if err != nil {
		slog.Debug("read socket", "error", err)
		return resp, err
	}
	if strings.Contains(string(data), "error") {
		return resp, errors.New(string(data))
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		slog.Debug("unmarshal socket response", "data", data, "error", err)
		return resp, err
	}
	return resp, nil
}
