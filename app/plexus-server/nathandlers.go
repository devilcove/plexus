package main

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

func devicePermissions(id string) *server.Permissions {
	return &server.Permissions{
		Publish: &server.SubjectPermission{
			Allow: []string{
				"checkin." + id,
				"update." + id,
				"config." + id,
				"leave." + id,
				"_INBOX.>",
			},
		},
		Subscribe: &server.SubjectPermission{
			Allow: []string{"networks.>", id, "_INBOX.>"},
		},
	}
}

func registerPermissions() *server.Permissions {
	return &server.Permissions{
		Publish: &server.SubjectPermission{
			Allow: []string{"register"},
		},
		Subscribe: &server.SubjectPermission{
			Allow: []string{"_INBOX.>"},
		},
	}
}

func registerHandler(request *plexus.ServerRegisterRequest) plexus.ServerResponse {
	slog.Debug("register request", "request", request)
	errResp := plexus.ServerResponse{Error: true}
	response := plexus.ServerResponse{}
	if err := saveNewPeer(request.Peer); err != nil {
		slog.Debug(err.Error())
		errResp.Message = err.Error()
		return errResp
	}
	if err := addNKeyUser(request.Peer); err != nil {
		errResp.Message = err.Error()
		slog.Debug(errResp.Message)
		return errResp
	}
	response.Message = "registration successful"
	return response
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

func addPeerToNetwork(peerID, network string) (plexus.Network, error) {
	netToUpdate, err := boltdb.Get[plexus.Network](network, networkTable)
	if err != nil {
		return netToUpdate, err
	}
	peer, err := boltdb.Get[plexus.Peer](peerID, peerTable)
	if err != nil {
		return netToUpdate, err
	}
	netPeer := plexus.NetworkPeer{}
	if err := encodedConn.Request(peer.WGPublicKey, plexus.DeviceUpdate{
		Action: plexus.SendListenPorts,
		Network: plexus.Network{
			Name: network,
		},
	}, &netPeer, natsTimeout); err != nil {
		return netToUpdate, err
	}
	netPeer.Endpoint = peer.Endpoint
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
	if err := encodedConn.Publish(peer.WGPublicKey, plexus.DeviceUpdate{
		Action:  plexus.JoinNetwork,
		Network: netToUpdate,
	}); err != nil {
		slog.Error("publish device update", "peer", netPeer.HostName, "error", err)
		return netToUpdate, err
	}
	slog.Debug("publish network update", "network", network, "update", update)
	if err := encodedConn.Publish("networks."+network, update); err != nil {
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
func processCheckin(data *plexus.CheckinData) plexus.ServerResponse {
	publishUpdate := false
	response := plexus.ServerResponse{}
	slog.Info("received checkin", "device", data.ID)
	peer, err := boltdb.Get[plexus.Peer](data.ID, peerTable)
	if err != nil {
		slog.Error("peer checkin", "error", err)
		response.Error = true
		response.Message = "no such peer"
		return response
	}
	peer.Updated = time.Now()
	peer.NatsConnected = true
	if peer.Version != data.Version {
		peer.Version = data.Version
	}
	//if peer.PublicListenPort != data.PublicListenPort {
	//	peer.PublicListenPort = data.PublicListenPort
	//	publishUpdate = true
	//}
	//if peer.ListenPort != data.ListenPort {
	//	peer.ListenPort = data.ListenPort
	//	publishUpdate = true
	//}
	if peer.Endpoint != data.Endpoint {
		peer.Endpoint = data.Endpoint
		publishUpdate = true
	}
	if err := boltdb.Save(peer, peer.WGPublicKey, peerTable); err != nil {
		slog.Error("peer checkin save", "error", err)
		response.Error = true
		response.Message = "could not save peer" + err.Error()
		return response
	}
	if publishUpdate {
		if err := publishNetworkPeerUpdate(peer); err != nil {
			slog.Error("checkin peer update", "error", err)
		}
	}
	processConnectionData(data)
	return plexus.ServerResponse{Message: "checkin processed"}
}

// configHandler handles requests for device configuration ie request published to config.<ID>
func configHandler(subject string) plexus.ServerResponse {
	response := plexus.ServerResponse{}
	peer := subject[7:]
	slog.Info("received config request", "peer", peer)
	networks, err := getNetworksForPeer(peer)
	if err != nil {
		response.Error = true
		response.Message = err.Error()
		return response
	}
	response.Networks = networks
	return response
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
		boltdb.Save(network, network.Name, networkTable)
	}
}

// processLeave handles leaving a network
func processLeave(request *plexus.AgentRequest) plexus.ServerResponse {
	errResponse := plexus.ServerResponse{Error: true}
	response := plexus.ServerResponse{}
	slog.Debug("leave handler", "peer", request.Peer.WGPublicKey, "network", request.Network)
	network, err := boltdb.Get[plexus.Network](request.Network, networkTable)
	if err != nil {
		slog.Error("get network to leave", "error", err)
		errResponse.Message = err.Error()
		return errResponse
	}
	found := false
	for i, peer := range network.Peers {
		if peer.WGPublicKey != request.Peer.WGPublicKey {
			continue
		}
		found = true
		network.Peers = slices.Delete(network.Peers, i, i+1)
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			slog.Error("save delete peer", "error", err)
			errResponse.Message = err.Error()
			return errResponse
		}
		update := plexus.NetworkUpdate{
			Action: plexus.DeletePeer,
			Peer:   peer,
		}
		slog.Debug("publishing network update for peer leaving network", "network", request.Network, "peer", request.Peer.WGPublicKey)
		if err := encodedConn.Publish("networks."+request.Network, update); err != nil {
			slog.Error("publish network update", "error", err)
			errResponse.Message = err.Error()
			return errResponse
		}
	}
	if !found {
		slog.Error("peer not found", "peer", request.Peer.WGPublicKey, "network", request.Network)
		errResponse.Message = fmt.Sprintf("error: %s not a part of %s network", request.Peer.WGPublicKey, request.Network)
		return errResponse
	}
	response.Message = fmt.Sprintf("%s deleted from %s network", request.Peer.WGPublicKey, request.Network)
	return response
}

func processUpdate(request *plexus.AgentRequest) plexus.ServerResponse {
	switch request.Action {
	case plexus.GetConfig:
		//handled in calling func
		return plexus.ServerResponse{}
	case plexus.Checkin:
		return processCheckin(&request.CheckinData)
	case plexus.JoinNetwork:
		return connectToNetwork(request)
	case plexus.Version:
		return serverVersion(request.Args)
	case plexus.LeaveServer:
		peer, err := discardPeer(request.Args)
		slog.Error("discard peer", "error", err)
		deletePeerFromBroker(peer.PubNkey)
		// with peer deleted, reply won't get sent so can return anything
		return plexus.ServerResponse{}
	case plexus.LeaveNetwork:
		return processLeave(request)
	default:
		return plexus.ServerResponse{
			Error:   true,
			Message: "invalid request action",
		}
	}
}

func connectToNetwork(request *plexus.AgentRequest) plexus.ServerResponse {
	errResponse := plexus.ServerResponse{Error: true}
	response := plexus.ServerResponse{}
	slog.Debug("join request", "peer", request.Peer.WGPublicKey, "network", request.Network)
	network, err := addPeerToNetwork(request.Peer.WGPublicKey, request.Network)
	if err != nil {
		errResponse.Message = err.Error()
		return errResponse
	}
	response.Networks = append(response.Networks, network)
	response.Message = fmt.Sprintf("%s added to network %s", request.Peer.WGPublicKey, request.Network)
	return response
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
				if err := encodedConn.Publish("networks."+network.Name, data); err != nil {
					slog.Error("publish network update", "error", err)
				}
			}
		}
	}
	return nil
}

func serverVersion(long string) plexus.ServerResponse {
	serverVersion := plexus.ServerVersion{
		Name:    config.FQDN,
		Version: version + ": ",
	}
	if long == "true" {
		info, _ := debug.ReadBuildInfo()
		for _, setting := range info.Settings {
			if strings.Contains(setting.Key, "vcs") {
				serverVersion.Version = serverVersion.Version + setting.Value + " "
			}
		}
	}
	return plexus.ServerResponse{Version: serverVersion}
}
