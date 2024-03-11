package agent

import (
	"errors"
	"log/slog"
	"slices"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func networkUpdates(subject string, update plexus.NetworkUpdate) {
	networkName := subject[9:]
	slog.Info("network update for", "network", networkName, "type", update.Action.String(), "peer", update.Peer)
	network, err := boltdb.Get[Network](networkName, networkTable)
	if err != nil {
		if errors.Is(err, boltdb.ErrNoResults) {
			slog.Info("received update for invalid network ... ignoring", "network", networkName)
			return
		}
		slog.Error("unable to read networks", "error", err)
		return
	}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("unable to read devices", "error", err)
		return
	}
	switch update.Action {
	case plexus.AddPeer:
		network.Peers = append(network.Peers, update.Peer)
		if err := addPeertoInterface(network.Interface, update.Peer); err != nil {
			slog.Error("add peer", "error", err)
		}
	case plexus.DeletePeer:
		if update.Peer.WGPublicKey == self.Peer.WGPublicKey {
			slog.Info("self delete --> delete network", "network", networkName)
			if err := boltdb.Delete[Network](network.Name, networkTable); err != nil {
				slog.Error("delete network", "error", err)
			}
			slog.Info("delete interface", "network", network.Name, "interface", network.Interface)
			if err := deleteInterface(network.Interface); err != nil {
				slog.Error("deleting interface", "interface", network.Interface, "error", err)
			}
			return
		}
		for i, oldpeer := range network.Peers {
			if oldpeer.WGPublicKey == update.Peer.WGPublicKey {
				network.Peers = slices.Delete(network.Peers, i, i)
			}
			if err := deletePeerFromInterface(network.Interface, update.Peer); err != nil {
				slog.Error("delete peer", "error", err)
			}
		}
	case plexus.UpdatePeer:
		for i, oldpeer := range network.Peers {
			if oldpeer.WGPublicKey == update.Peer.WGPublicKey {
				network.Peers = slices.Replace(network.Peers, i, i+1, update.Peer)
			}
		}
		if err := replacePeerInInterface(network.Interface, update.Peer); err != nil {
			slog.Error("replace peer", "error", err)
		}

	case plexus.AddRelay:
		newPeers := []plexus.NetworkPeer{}
		for _, existing := range network.Peers {
			if existing.WGPublicKey == update.Peer.WGPublicKey {
				newPeers = append(newPeers, update.Peer)
				continue
			}
			if slices.Contains(update.Peer.RelayedPeers, existing.WGPublicKey) {
				existing.IsRelayed = true
			}
			newPeers = append(newPeers, existing)
		}
		network.Peers = newPeers
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("update network with relayed peers", "error", err)
		}
		if err := resetPeersOnNetworkInterface(self, network); err != nil {
			slog.Error("add relay:restart interface", "network", network.Name, "error", err)
		}

	case plexus.DeleteRelay:
		oldRelay := update.Peer
		newPeers := []plexus.NetworkPeer{}
		for _, existing := range network.Peers {
			if existing.WGPublicKey == oldRelay.WGPublicKey {
				existing.IsRelay = false
				existing.RelayedPeers = []string{}
			}
			if slices.Contains(oldRelay.RelayedPeers, existing.WGPublicKey) {
				existing.IsRelayed = false
			}
			newPeers = append(newPeers, existing)
		}
		network.Peers = newPeers
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("remove relay: save network", "network", network.Name, "error", err)
		}
		if err := resetPeersOnNetworkInterface(self, network); err != nil {
			slog.Error("delete relay:restart interface", "network", network.Name, "error", err)
		}

	case plexus.DeleteNetwork:
		slog.Info("delete network")
		if err := boltdb.Delete[Network](network.Name, networkTable); err != nil {
			slog.Error("delete network", "error", err)
		}
		if err := deleteInterface(network.Interface); err != nil {
			slog.Error("delete interfadce", "interface", network.Interface, "errror", err)
		}
		return
	default:
		slog.Info("invalid network update type")
		return
	}
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		slog.Error("update network during", "command", update.Action, "error", err)
	}
}
