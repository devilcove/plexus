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
	request := &plexus.ListenPortRequest{
		Network: network,
	}
	bytes, err := json.Marshal(request)
	msg, err := natsConn.Request(plexus.Update+id+plexus.SendListenPorts, bytes, natsTimeout)
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
