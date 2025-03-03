package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/devilcove/plexus"
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
