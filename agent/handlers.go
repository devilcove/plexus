package agent

import (
	"errors"
	"log/slog"
	"slices"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

// server handlers
func networkUpdates(subject string, update plexus.NetworkUpdate) {
	networkName := subject[9:]
	slog.Info("network update for", "network", networkName, "action", update.Action, "peer", update.Peer)
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
		slog.Debug("add peer")
		for _, peer := range network.Peers {
			if peer.WGPublicKey == update.Peer.WGPublicKey {
				slog.Error("peer already exists", "network", networkName, "peer", update.Peer.HostName, "id", update.Peer.WGPublicKey)
				return
			}
		}
		network.Peers = append(network.Peers, update.Peer)
		if err := addPeertoInterface(network.Interface, update.Peer); err != nil {
			slog.Error("add peer", "error", err)
		}
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("update network -- add peer", "error", err)
		}
	case plexus.DeletePeer:
		slog.Debug("delete peer")
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
		found := false
		for i, oldpeer := range network.Peers {
			if oldpeer.WGPublicKey == update.Peer.WGPublicKey {
				slog.Debug("found peer to delete")
				found = true
				network.Peers = slices.Delete(network.Peers, i, i+1)
			}
			if err := deletePeerFromInterface(network.Interface, update.Peer); err != nil {
				slog.Error("delete peer", "error", err)
			}
			if found {
				break
			}
		}
		if !found {
			slog.Error("peer does not exist", "network", networkName, "peer", update.Peer.HostName, "id", update.Peer.WGPublicKey)
			return
		}
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("update network -- delete peer", "error", err)
		}
	case plexus.UpdatePeer:
		slog.Debug("update peer")
		found := false
		for i, oldpeer := range network.Peers {
			if oldpeer.WGPublicKey == update.Peer.WGPublicKey {
				network.Peers = slices.Replace(network.Peers, i, i+1, update.Peer)
				found = true
			}
			if found {
				break
			}
		}
		if !found {
			slog.Error("peer does not exist", "network", networkName, "peer", update.Peer.HostName, "id", update.Peer.WGPublicKey)
			return
		}
		if err := replacePeerInInterface(network.Interface, update.Peer); err != nil {
			slog.Error("replace peer", "error", err)
		}
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("update network -- delete peer", "error", err)
		}
	case plexus.AddRelay:
		slog.Debug("add relay")
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
		slog.Debug("delete relay")
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
		slog.Debug("delete network")
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
}

func processStatus() StatusResponse {
	networks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		slog.Error("get networks", "error", err)
	}
	response := StatusResponse{Networks: networks}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
	}
	response.Server = self.Server

	ec := serverConn.Load()
	if ec == nil {
		response.Connected = false
	} else {
		response.Connected = ec.Conn.IsConnected()
	}
	return response
}

func processJoin(request *plexus.JoinRequest) plexus.JoinResponse {
	networks := []plexus.Network{}
	slog.Debug("join", "network", request.Network)
	response := plexus.JoinResponse{}
	_, err := boltdb.Get[Network](request.Network, networkTable)
	if err == nil {
		slog.Warn("already connected to network")
		return plexus.JoinResponse{Message: "error: already connected to network"}
	}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		return plexus.JoinResponse{Message: "error:" + err.Error()}
	}
	request.Peer = self.Peer
	slog.Debug("sending join request to server")
	serverEC := serverConn.Load()
	if serverEC == nil {
		return plexus.JoinResponse{Message: "not connnected to server"}
	}
	if err := serverEC.Request(self.WGPublicKey+plexus.JoinNetwork, request, &response, NatsTimeout); err != nil {
		slog.Debug(err.Error())
		return plexus.JoinResponse{Message: "error:" + err.Error()}
	}
	addNewNetworks(self, append(networks, response.Network))
	return response
}

func processLeave(request *plexus.LeaveRequest) plexus.MessageResponse {
	response := plexus.MessageResponse{}
	slog.Debug("leave", "network", request.Network)
	network, err := boltdb.Get[Network](request.Network, networkTable)
	if err != nil {
		slog.Debug(err.Error())
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	serverEC := serverConn.Load()
	if serverEC != nil {
		if err := serverEC.Request(self.WGPublicKey+plexus.LeaveNetwork, request, &response, NatsTimeout); err != nil {
			slog.Debug(err.Error())
			return plexus.MessageResponse{Message: "error: " + err.Error()}
		}
	} else {
		return plexus.MessageResponse{Message: "not connected to server"}
	}
	if err := deleteInterface(network.Interface); err != nil {
		slog.Debug("delete interface", "error", err)
		return plexus.MessageResponse{Message: "failed to delete interface: " + err.Error()}
	}
	if err := boltdb.Delete[Network](request.Network, networkTable); err != nil {
		slog.Debug(err.Error())
		return plexus.MessageResponse{Message: "failed to delete network: " + err.Error()}
	}
	slog.Debug("leave complete")
	return response
}

func processLeaveServer() plexus.MessageResponse {
	response := plexus.MessageResponse{}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	if self.Server == "" {
		return plexus.MessageResponse{Message: "error: not connected to server"}
	}
	serverEC := serverConn.Load()
	if serverEC != nil {
		if err := serverEC.Publish(self.WGPublicKey+plexus.LeaveServer, nil); err != nil {
			return plexus.MessageResponse{Message: "error: " + err.Error()}
		}
	}
	serverConn.Store(nil)
	self.Server = ""
	if err := boltdb.Save(self, "self", deviceTable); err != nil {
		slog.Error("save device", "error", err)
	}
	return response
}

func processReload() (plexus.NetworkResponse, error) {
	response := plexus.NetworkResponse{}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
		return response, err
	}
	serverEC := serverConn.Load()
	if serverEC == nil {
		return response, errors.New("not connected")
	}
	if err := serverEC.Request(self.WGPublicKey+plexus.Reload, nil, &response, NatsTimeout); err != nil {
		return response, err
	}
	return response, nil
}
