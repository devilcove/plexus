package agent

import (
	"errors"
	"log/slog"
	"slices"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func deleteServer(server string) {
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("unable to read device", "error", err)
		return
	}
	for i, serv := range self.Servers {
		if serv == server {
			self.Servers = slices.Delete(self.Servers, i, i+1)
		}
	}
	if err := boltdb.Save(self, "self", "devices"); err != nil {
		slog.Error("save device", "error", err)
	}
}

func networkUpdates(subject string, update plexus.NetworkUpdate) {
	networkName := subject[9:]
	slog.Info("network update for", "network", networkName, "msg", update)
	network, err := boltdb.Get[plexus.Network](networkName, "networks")
	if err != nil {
		if errors.Is(err, boltdb.ErrNoResults) {
			slog.Info("received update for invalid network", "network", network)
			return
		}
		slog.Error("unable to read networks", "error", err)
		return
	}
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("unable to read devices", "error", err)
		return
	}
	switch update.Type {
	case plexus.AddPeer:
		slog.Info("addpeer")
		network.Peers = append(network.Peers, update.Peer)
		if err := addPeertoInterface(networkMap[network.Name].Interface, update.Peer); err != nil {
			slog.Error("add peer", "error", err)
		}
	case plexus.DeletePeer:
		slog.Info("delete peer from network", "peer address", update.Peer.Address, "network", networkName)
		if update.Peer.WGPublicKey == self.Peer.WGPublicKey {
			slog.Info("self delete --> delete network", "network", networkName)
			networkMap[network.Name].Channel <- true
			if err := boltdb.Delete[plexus.Network](network.Name, "networks"); err != nil {
				slog.Error("delete network", "error", err)
			}
			slog.Info("delete interface", "network", network.Name, "interface", networkMap[network.Name].Interface)
			deleteInterface(networkMap[network.Name].Interface)
			return
		}
		for i, oldpeer := range network.Peers {
			if oldpeer.WGPublicKey == update.Peer.WGPublicKey {
				network.Peers = slices.Delete(network.Peers, i, i)
			}
			if err := deletePeerFromInterface(networkMap[network.Name].Interface, update.Peer); err != nil {
				slog.Error("delete peer", "error", err)
			}
		}
	case plexus.UpdatePeer:
		slog.Info("update peer")
		for i, oldpeer := range network.Peers {
			if oldpeer.WGPublicKey == update.Peer.WGPublicKey {
				network.Peers = slices.Replace(network.Peers, i, i+1, update.Peer)
			}
		}
		if err := replacePeerInInterface(networkMap[networkName].Interface, update.Peer); err != nil {
			slog.Error("replace peer", "error", err)
		}
	case plexus.AddRelay:
		slog.Info("add relay")
		if err := addRelayToInterface(networkMap[networkName].Interface, update.Peer); err != nil {
			slog.Error("addRelay", "error", err)
		}
	case plexus.DeleteNetork:
		slog.Info("delete network")
		networkMap[network.Name].Channel <- true
		if err := boltdb.Delete[plexus.Network](network.Name, "networks"); err != nil {
			slog.Error("delete network", "error", err)
		}
		deleteInterface(networkMap[network.Name].Interface)
		return
	default:
		slog.Info("invalid network update type")
		return
	}
	if err := boltdb.Save(network, network.Name, "networks"); err != nil {
		slog.Error("update network during", "command", update.Type, "error", err)
	}
}
