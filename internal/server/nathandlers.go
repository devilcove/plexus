package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
)

func registerHandler(request *plexus.ServerRegisterRequest) plexus.MessageResponse {
	slog.Debug("register request", "request", request)
	if err := saveNewPeer(request.Peer); err != nil {
		slog.Debug(err.Error())
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	if err := addNKeyUser(request.Peer); err != nil {
		slog.Debug(err.Error())
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	return plexus.MessageResponse{Message: "registration successful"}
}

func saveNewPeer(peer plexus.Peer) error {
	if _, err := boltdb.Get[plexus.Peer](peer.WGPublicKey, peerTable); err == nil {
		return errors.New("peer exists")
	}
	// save new peer(device)
	if err := boltdb.Save(peer, peer.WGPublicKey, peerTable); err != nil {
		slog.Debug("unable to save new peer", "error", err)
		return err
	}
	return nil
}

func addNKeyUser(peer plexus.Peer) error {
	for _, nkeys := range natsOptions.Nkeys {
		slog.Debug("checking device nKeys", nkeys.Nkey, peer.PubNkey)
		if nkeys.Nkey == peer.PubNkey {
			slog.Error("nkey user exist")
			return nil
		}
	}
	device := &server.NkeyUser{
		Nkey:        peer.PubNkey,
		Permissions: devicePermissions(peer.WGPublicKey),
	}
	natsOptions.Nkeys = append(natsOptions.Nkeys, device)
	if err := natServer.ReloadOptions(natsOptions); err != nil {
		return err
	}
	return nil
}

func addPeerToNetwork(peerID, network string, listenPort, publicListenPort int) (plexus.Network, error) {
	netToUpdate, err := boltdb.Get[plexus.Network](network, networkTable)
	if err != nil {
		return netToUpdate, err
	}
	peer, err := boltdb.Get[plexus.Peer](peerID, peerTable)
	if err != nil {
		return netToUpdate, err
	}
	netPeer := plexus.NetworkPeer{
		WGPublicKey:      peer.WGPublicKey,
		HostName:         peer.Name,
		ListenPort:       listenPort,
		PublicListenPort: publicListenPort,
		Endpoint:         peer.Endpoint,
	}
	// check if peer is already part of network
	for _, existing := range netToUpdate.Peers {
		if existing.WGPublicKey == peer.WGPublicKey {
			return netToUpdate, fmt.Errorf("peer exists in network %s", network)
		}
	}
	addr, err := getNextIP(netToUpdate)
	if err != nil {
		return netToUpdate, fmt.Errorf("unable to get ip for peer %s %s %v", peer.WGPublicKey, network, err)
	}
	slog.Debug("setting ip to", "ip", addr)
	netPeer.Address = net.IPNet{
		IP:   addr,
		Mask: netToUpdate.Net.Mask,
	}
	update := plexus.NetworkUpdate{
		Action: plexus.AddPeer,
		Peer:   netPeer,
	}
	netToUpdate.Peers = append(netToUpdate.Peers, update.Peer)
	if err := boltdb.Save(netToUpdate, netToUpdate.Name, networkTable); err != nil {
		slog.Error("save updated network", "error", err)
		return netToUpdate, err
	}
	slog.Debug("publish device update", "name", netPeer.HostName)
	if err := eConn.Publish(plexus.Update+peer.WGPublicKey+plexus.JoinNetwork, plexus.DeviceUpdate{
		Action:  plexus.JoinNetwork,
		Network: netToUpdate,
	}); err != nil {
		slog.Error("publish device update", "peer", netPeer.HostName, "error", err)
		return netToUpdate, err
	}
	slog.Debug("publish network update", "network", network, "update", update)
	if err := eConn.Publish(plexus.Networks+network, update); err != nil {
		slog.Error("publish new peer", "error", err)
		return netToUpdate, err
	}
	return netToUpdate, nil
}

func getNextIP(network plexus.Network) (net.IP, error) {
	taken := make(map[string]bool)
	for _, peer := range network.Peers {
		taken[peer.Address.IP.String()] = true
	}
	slog.Debug("getnextIP", "network", network)
	slog.Debug("getNextIP", "taken", taken)
	slog.Debug("getNextIP", "net", network.Net)
	ipnet := iplib.Net4FromStr(network.Net.String())
	ipToCheck := ipnet.FirstAddress()
	broadcast := ipnet.BroadcastAddress()
	for {
		slog.Debug("checking", "ip", ipToCheck, "network", network.Net)
		_, ok := taken[ipToCheck.String()]
		if !ok {
			slog.Debug("found available ip", "ip", ipToCheck, "taken", taken)
			break
		}
		next := iplib.NextIP(ipToCheck)
		if next.Equal(broadcast) {
			return net.IP{}, errors.New("no addresses available")
		}
		ipToCheck = next
	}
	return ipToCheck, nil
}

// processCheckin handle messages published to checkin.<ID>
func processCheckin(data *plexus.CheckinData) plexus.MessageResponse {
	publishUpdate := false
	response := plexus.MessageResponse{}
	slog.Info("received checkin", "device", data.ID)
	peer, err := boltdb.Get[plexus.Peer](data.ID, peerTable)
	if err != nil {
		slog.Error("peer checkin", "error", err)
		response.Message = "no such peer"
		return response
	}
	peer.Updated = time.Now()
	peer.NatsConnected = true
	if peer.Version != data.Version {
		peer.Version = data.Version
	}
	if !peer.Endpoint.Equal(data.Endpoint) {
		peer.Endpoint = data.Endpoint
		publishUpdate = true
	}
	if err := boltdb.Save(peer, peer.WGPublicKey, peerTable); err != nil {
		slog.Error("peer checkin save", "error", err)
		response.Message = "could not save peer" + err.Error()
		return response
	}
	if publishUpdate {
		if err := publishNetworkPeerUpdate(peer); err != nil {
			slog.Error("checkin peer update", "error", err)
		}
	}
	processConnectionData(data)
	return plexus.MessageResponse{Message: "checkin processed"}
}

// configHandler handles requests for device configuration ie request published to config.<ID>
func processReload(id string) plexus.NetworkResponse {
	slog.Debug("received reload request", "peer", id)
	networks, err := getNetworksForPeer(id)
	if err != nil {
		return plexus.NetworkResponse{Message: "error: " + err.Error()}
	}
	return plexus.NetworkResponse{Networks: networks}
}

// processConnectionData handles connectivity (nats, handshakes) stats
func processConnectionData(data *plexus.CheckinData) {
	slog.Debug("received connectivity stats", "device", data.ID)
	for _, conn := range data.Connections {
		network, err := boltdb.Get[plexus.Network](conn.Network, networkTable)
		if err != nil {
			slog.Error("connectivity data received for invalid network", "network", conn.Network)
			continue
		}
		updatedPeers := []plexus.NetworkPeer{}
		for _, peer := range network.Peers {
			if peer.WGPublicKey == data.ID {
				peer.Connectivity = conn.Connectivity
				peer.NatsConnected = true
			}
			updatedPeers = append(updatedPeers, peer)
		}
		network.Peers = updatedPeers
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("save peers", "error", err)
		}
	}
}

// processLeave handles leaving a network
func processLeave(id string, request *plexus.LeaveRequest) plexus.MessageResponse {
	slog.Debug("leave handler", "peer", id, "network", request.Network)
	network, err := boltdb.Get[plexus.Network](request.Network, networkTable)
	if err != nil {
		slog.Error("get network to leave", "error", err)
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	found := false
	for i, peer := range network.Peers {
		if peer.WGPublicKey != id {
			continue
		}
		found = true
		network.Peers = slices.Delete(network.Peers, i, i+1)
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("save delete peer", "error", err)
			return plexus.MessageResponse{Message: "error: " + err.Error()}
		}
		update := plexus.NetworkUpdate{
			Action: plexus.DeletePeer,
			Peer:   peer,
		}
		slog.Debug("publishing network update for peer leaving network", "network", request.Network, "peer", id)
		if err := eConn.Publish(plexus.Networks+request.Network, update); err != nil {
			slog.Error("publish network update", "error", err)
			return plexus.MessageResponse{Message: "error: " + err.Error()}
		}
	}
	if !found {
		slog.Error("peer not found", "peer", id, "network", request.Network)
		return plexus.MessageResponse{Message: "error: peer not in network"}
	}
	return plexus.MessageResponse{
		Message: fmt.Sprintf("%s deleted from %s network", id, request.Network),
	}
}

func publishNetworkPeerUpdate(peer plexus.Peer) error {
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		return err
	}
	for i, network := range networks {
		for j, netPeer := range network.Peers {
			if netPeer.WGPublicKey == peer.WGPublicKey {
				//netPeer.PublicListenPort = peer.PublicListenPort
				netPeer.Endpoint = peer.Endpoint
				networks[i].Peers[j] = netPeer
				data := plexus.NetworkUpdate{
					Action: plexus.UpdatePeer,
					Peer:   netPeer,
				}
				if err := eConn.Publish(plexus.Networks+network.Name, data); err != nil {
					slog.Error("publish network update", "error", err)
				}
			}
		}
	}
	return nil
}

func serverVersion() plexus.VersionResponse {
	serverVersion := version + ": "
	info, _ := debug.ReadBuildInfo()
	for _, setting := range info.Settings {
		if strings.Contains(setting.Key, "vcs") {
			serverVersion = serverVersion + setting.Value + " "
		}
	}
	return plexus.VersionResponse{Server: serverVersion}
}

func processJoin(id string, request *plexus.JoinRequest) plexus.JoinResponse {
	if id != request.Peer.WGPublicKey {
		return plexus.JoinResponse{Message: "peer id does not match subject"}
	}
	network, err := addPeerToNetwork(request.WGPublicKey, request.Network,
		request.ListenPort, request.PublicListenPort)
	if err != nil {
		return plexus.JoinResponse{Message: err.Error()}
	}
	return plexus.JoinResponse{
		Message: fmt.Sprintf("peer added to network %s", request.Network),
		Network: network,
	}
}

func processLeaveServer(id string) error {
	slog.Debug("remove peer", "peer", id)
	peer, err := discardPeer(id)
	if err != nil {
		slog.Debug(err.Error())
		return err
	}
	deletePeerFromBroker(peer.PubNkey)
	return nil
}