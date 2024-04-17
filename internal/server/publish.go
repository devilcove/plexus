package server

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/devilcove/plexus"
)

func getListenPorts(id, network string) (int, int, error) {
	response := plexus.ListenPortResponse{}
	slog.Debug("requesting listen port from peer", "id", id)
	if err := eConn.Request(plexus.Update+id+plexus.SendListenPorts, plexus.ListenPortRequest{
		Network: network,
	}, &response, natsTimeout); err != nil {
		return 0, 0, err
	}
	if strings.Contains(response.Message, "error") {
		return 0, 0, errors.New(response.Message)
	}
	return response.ListenPort, response.PublicListenPort, nil
}
