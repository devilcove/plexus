package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
)

func getListenPorts(id, network string) (int, int, error) {
	response := &plexus.ListenPortResponse{}
	slog.Debug("requesting listen port from peer", "id", id)
	request, err := json.Marshal(plexus.ListenPortRequest{
		Network: network,
	})
	if err != nil {
		slog.Error("invalid request", "error", err, "network", network)
	}
	msg, err := natsConn.Request(plexus.Update+id+plexus.SendListenPorts, request, natsTimeout)
	if err != nil {
		return 0, 0, err
	}
	if err := json.Unmarshal(msg.Data, response); err != nil {
		return 0, 0, err
	}
	if strings.Contains(response.Message, "error") {
		return 0, 0, errors.New(response.Message)
	}
	return response.ListenPort, response.PublicListenPort, nil
}

func publishErrorMessage(conn *nats.Conn, subj string, msg string, err error) {
	response, err := json.Marshal(plexus.ErrorResponse{
		Message: msg,
		Error:   err,
	})
	if err != nil {
		slog.Error("invalid message respone", "error", err)
	}
	if err := conn.Publish(subj, response); err != nil {
		slog.Error("publish error", "error", err)
	}
}

func publishMessage(conn *nats.Conn, subj string, data any) {
	bytes, err := json.Marshal(data)
	if err != nil {
		slog.Error("invalid message data", "error", err, "data", data)
	}
	if err := conn.Publish(subj, bytes); err != nil {
		slog.Error("publish msg", "connection", conn.Opts.Name, "subject", subj, "data", data)
	}
}
