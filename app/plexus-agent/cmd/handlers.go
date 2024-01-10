package cmd

import (
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
)

func networkUpdates(msg *nats.Msg) {
	parts := strings.Split(msg.Subject, ".")
	if len(parts) < 2 {
		slog.Error("invalid msg subject", "subject", msg.Subject)
		return
	}
	networkName := parts[1]
	command := parts[2]
	slog.Info("network update for", "network", networkName, "command", command)
	network, err := boltdb.Get[plexus.Network](networkName, "networks")
	if err != nil {
		if errors.Is(err, boltdb.ErrNoResults) {
			slog.Info("received update for invalid network")
			return
		}
		slog.Error("unable to read networks", "error", err)
		return
	}
	peer := plexus.NetworkPeer{}
	if err := json.Unmarshal(msg.Data, &peer); err != nil {
		slog.Error("network update", "error", err)
		return
	}
	switch command {
	case "newPeer":
		network.Peers = append(network.Peers, peer)
	case "peerUpdate":
		for i, oldpeer := range network.Peers {
			if oldpeer.WGPublicKey == peer.WGPublicKey {
				network.Peers = slices.Replace(network.Peers, i, i+1, peer)
			}
		}
	case "deletePeer":
		for i, oldpeer := range network.Peers {
			if oldpeer.WGPublicKey == peer.WGPublicKey {
				network.Peers = slices.Delete(network.Peers, i, i)
			}
		}
	}
	if err := boltdb.Save(network, network.Name, "networks"); err != nil {
		slog.Error("error process command", "command", command, "error", err)
	}
}
