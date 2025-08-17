package agent

import (
	"encoding/json"
	"errors"
	"log/slog"
	"slices"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
)

// server handlers.
func networkUpdates(msg *nats.Msg) {
	// func networkUpdates(subject string, update plexus.NetworkUpdate) {
	networkName := msg.Subject[9:]
	update := &plexus.NetworkUpdate{}
	if err := json.Unmarshal(msg.Data, update); err != nil {
		slog.Error("invalid network update", "error", err, "data", string(msg.Data))
		return
	}
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
	wg, err := plexus.Get(network.Interface)
	if err != nil {
		slog.Error("get wireguard interface", "interface", network.Interface, "error", err)
		return
	}
	switch update.Action {
	case plexus.AddPeer:
		processAddPeer(network, update, wg)
	case plexus.DeletePeer:
		processDeletePeer(network, update, self, wg)
	case plexus.UpdatePeer:
		processUpdatePeer(network, update, wg)
	case plexus.AddRelay:
		processAddRelay(network, update, self)
	case plexus.DeleteRelay:
		processDeleteRelay(network, update, self)

	case plexus.DeleteNetwork:
		processDeleteNetwork(network)
	default:
		slog.Info("invalid network update type")
	}
}

func processStatus() []byte {
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
		response.Connected = ec.IsConnected()
	}
	bytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("encode status response", "error", err)
	}
	return bytes
}

func serviceJoin(in []byte) []byte {
	request := &plexus.JoinRequest{}
	if err := json.Unmarshal(in, request); err != nil {
		slog.Error("invalid join request", "error", err, "data", string(in))
		return []byte{}
	}
	response := processJoin(request)
	bytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("invalid join response", "error", err, "data", response)
	}
	return bytes
}

func processJoin(request *plexus.JoinRequest) plexus.JoinResponse {
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
	tempPeer, err := getNewListenPorts(request.Network)
	if err != nil {
		slog.Error("unable to obtain listen port", "error", err)
		return plexus.JoinResponse{Message: "unable to obtain listen port " + err.Error()}
	}
	request.ListenPort = tempPeer.ListenPort
	request.PublicListenPort = tempPeer.PublicListenPort
	slog.Debug("sending join request to server")
	serverConn := serverConn.Load()
	if serverConn == nil {
		return plexus.JoinResponse{Message: "not connnected to server"}
	}
	if err := Request(serverConn, self.WGPublicKey+plexus.JoinNetwork, request, &response, NatsTimeout); err != nil {
		slog.Debug(err.Error())
		return plexus.JoinResponse{Message: "error:" + err.Error()}
	}
	return response
}

func handleLeave(in []byte) []byte {
	request := &plexus.LeaveRequest{}
	if err := json.Unmarshal(in, request); err != nil {
		slog.Error("invalid leave request", "error", err, "data", string(in))
		return []byte{}
	}
	response := processLeave(request)
	bytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("invalid leave response", "error", err, "data", response)
	}
	return bytes
}

func processLeave(request *plexus.LeaveRequest) plexus.MessageResponse {
	response := plexus.MessageResponse{}
	slog.Debug("leave", "network", request.Network)
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	serverConn := serverConn.Load()
	if serverConn != nil {
		if err := Request(serverConn, self.WGPublicKey+plexus.LeaveNetwork, request, &response, NatsTimeout); err != nil {
			slog.Debug(err.Error())
			return plexus.MessageResponse{Message: "error: " + err.Error()}
		}
	} else {
		return plexus.MessageResponse{Message: "not connected to server"}
	}
	slog.Debug("leave complete")
	return response
}

func handleLeaveServer() ([]byte, error) {
	response := processLeaveServer()
	return json.Marshal(response)
}

func processLeaveServer() plexus.MessageResponse {
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Debug(err.Error())
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	if self.Server == "" {
		return plexus.MessageResponse{Message: "error: not connected to server"}
	}
	natsConn := serverConn.Load()
	if natsConn != nil {
		if err := natsConn.Publish(self.WGPublicKey+plexus.LeaveServer, nil); err != nil {
			return plexus.MessageResponse{Message: "error: " + err.Error()}
		}
	}
	serverConn.Store(nil)
	self.Server = ""
	if err := boltdb.Save(self, "self", deviceTable); err != nil {
		slog.Error("save device", "error", err)
	}
	return plexus.MessageResponse{Message: "left server " + self.Server}
}

func processReload() (plexus.NetworkResponse, error) {
	response := plexus.NetworkResponse{}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
		return response, err
	}
	serverConn := serverConn.Load()
	if serverConn == nil {
		return response, errors.New("not connected")
	}
	if err := Request(serverConn, self.WGPublicKey+plexus.Reload, nil, &response, NatsTimeout); err != nil {
		return response, err
	}
	return response, nil
}

func processAddPeer(network Network, update *plexus.NetworkUpdate, wg *plexus.Wireguard) {
	slog.Debug("add peer")
	for _, peer := range network.Peers {
		if peer.WGPublicKey == update.Peer.WGPublicKey {
			slog.Error("peer already exists", "network", network.Name, "peer", update.Peer.HostName, "id",
				update.Peer.WGPublicKey)
			return
		}
	}
	if update.Peer.PrivateEndpoint != nil {
		if connectToPublicEndpoint(update.Peer) {
			update.Peer.UsePrivateEndpoint = true
		}
	}
	network.Peers = append(network.Peers, update.Peer)
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		slog.Error("update network -- add peer", "error", err)
	}
	wgPeer, err := convertPeerToWG(update.Peer, network.Peers)
	if err != nil {
		slog.Error("convert peer", "peer", update.Peer.HostName, "error", err)
		return
	}
	slog.Debug("adding wg peer", "key", wgPeer.PublicKey, "allowedIPs", wgPeer.AllowedIPs)
	wg.AddPeer(wgPeer)
	if err := wg.Apply(); err != nil {
		slog.Error("apply wg config", "error", err)
	}
}

func processDeletePeer(network Network, update *plexus.NetworkUpdate, self Device, wg *plexus.Wireguard) {
	slog.Debug("delete peer")
	if update.Peer.WGPublicKey == self.WGPublicKey {
		slog.Info("self delete --> delete network", "network", network.Name)
		if err := boltdb.Delete[Network](network.Name, networkTable); err != nil {
			slog.Error("delete network", "error", err)
		}
		slog.Info("delete interface", "network", network.Name, "interface", network.Interface)
		if err := deleteInterface(network.Interface); err != nil {
			slog.Error("deleting interface", "interface", network.Interface, "error", err)
		}
		return
	}
	wg.DeletePeer(update.Peer.WGPublicKey)
	found := false
	for i, oldpeer := range network.Peers {
		if oldpeer.WGPublicKey == update.Peer.WGPublicKey {
			slog.Debug("found peer to delete")
			found = true
			network.Peers = slices.Delete(network.Peers, i, i+1)
			break
		}
	}
	if !found {
		slog.Error("peer does not exist", "network", network.Name, "peer", update.Peer.HostName,
			"id", update.Peer.WGPublicKey)
		return
	}
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		slog.Error("update network -- delete peer", "error", err)
	}
	if err := wg.Apply(); err != nil {
		slog.Error("apply wg config", "error", err)
	}
}

func processUpdatePeer(network Network, update *plexus.NetworkUpdate, wg *plexus.Wireguard) {
	slog.Debug("update peer")
	found := false
	for i, oldpeer := range network.Peers {
		if oldpeer.WGPublicKey == update.Peer.WGPublicKey {
			if update.Peer.PrivateEndpoint != nil {
				if connectToPublicEndpoint(update.Peer) {
					update.Peer.UsePrivateEndpoint = true
				}
			}
			network.Peers = slices.Replace(network.Peers, i, i+1, update.Peer)
			found = true
			break
		}
	}
	if !found {
		slog.Error("peer does not exist", "network", network.Name, "peer", update.Peer.HostName,
			"id", update.Peer.WGPublicKey)
		return
	}
	wgPeer, err := convertPeerToWG(update.Peer, network.Peers)
	if err != nil {
		slog.Error("convert to WG peer", "error", err)
		return
	}
	wg.ReplacePeer(wgPeer)
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		slog.Error("update network -- update peer", "error", err)
	}
	if err := wg.Apply(); err != nil {
		slog.Error("apply wg config", "error", err)
	}
}

func processAddRelay(network Network, update *plexus.NetworkUpdate, self Device) {
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
}

func processDeleteRelay(network Network, update *plexus.NetworkUpdate, self Device) {
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
}

func processDeleteNetwork(network Network) {
	slog.Debug("delete network")
	slog.Info("delete network")
	if err := boltdb.Delete[Network](network.Name, networkTable); err != nil {
		slog.Error("delete network", "error", err)
	}
	if err := deleteInterface(network.Interface); err != nil {
		slog.Error("delete interfadce", "interface", network.Interface, "errror", err)
	}
}

func deleteRouter(msg *nats.Msg, id string) {
	data := &plexus.NetworkPeer{}
	if err := json.Unmarshal(msg.Data, data); err != nil {
		slog.Error("invalid network peer", "error", err, "data", string(msg.Data))
	}

	if data.WGPublicKey != id {
		slog.Error("add router wrong id", "me", id, "router", data.WGPublicKey)
		return
	}
	if err := delNat(); err != nil {
		slog.Error("delete nat", "error", err)
	}
	if err := delVirtualSubnet(); err != nil {
		slog.Error("delete virtual subnet", "error", err)
	}
}

func addRouter(msg *nats.Msg, id string) {
	data := &plexus.NetworkPeer{}
	if err := json.Unmarshal(msg.Data, data); err != nil {
		slog.Error("invalid network peer", "error", err, "data", string(msg.Data))
	}
	if data.WGPublicKey != id {
		slog.Error("add router wrong id", "me", id, "router", data.WGPublicKey)
		return
	}
	if !data.IsSubnetRouter {
		return
	}
	slog.Debug("adding subnet router")
	if data.UseNat {
		if err := addNat(); err != nil {
			slog.Error("add nat", "error", err)
		}
	}
	if data.UseVirtSubnet {
		if err := addVirtualSubnet(data.VirtSubnet, data.Subnet); err != nil {
			slog.Error("add virtual subnet", "error", err)
		}
	}
}

func sendListenPorts(msg *nats.Msg, serverConn *nats.Conn) {
	data := &plexus.ListenPortRequest{}
	if err := json.Unmarshal(msg.Data, data); err != nil {
		slog.Error("invalid server listen port request", "error", err, "data", string(msg.Data))
	}
	slog.Info("new listen ports", "network", data.Network)
	response, err := getNewListenPorts(data.Network)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	bytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("invalid listen port response", "error", err, "data", response)
	}
	if err := serverConn.Publish(msg.Reply, bytes); err != nil {
		slog.Error("publish reply to SendListenPorts", "error", err)
	}
	slog.Debug("sent listenports to server", "public", response.PublicListenPort, "private", response.ListenPort)
}

func joinNetwork(msg *nats.Msg, self Device) {
	data := &plexus.ServerJoinRequest{}
	if err := json.Unmarshal(msg.Data, data); err != nil {
		slog.Error("invalid server join request", "error", err, "data", string(msg.Data))
	}
	slog.Info("join network", "network", data.Network)
	network, err := saveServerNetwork(data.Network)
	if err != nil {
		slog.Error("save network", "error", err)
		return
	}
	if err := startInterface(self, network); err != nil {
		slog.Error("error starting interface", "interface", network.Interface, "network", network.Name, "error", err)
		return
	}
}
