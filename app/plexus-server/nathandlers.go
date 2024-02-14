package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func devicePermissions(id string) *server.Permissions {
	return &server.Permissions{
		Publish: &server.SubjectPermission{
			Allow: []string{
				"checkin." + id,
				"update." + id,
				"config." + id,
				"connectivity." + id,
				"leave." + id,
			},
		},
		Subscribe: &server.SubjectPermission{
			Allow: []string{"networks.>", "_INBOX.>"},
		},
	}
}

func joinHandler(request *plexus.JoinRequest) plexus.NetworkResponse {
	slog.Debug("join request", "request", request)
	errResp := plexus.NetworkResponse{Error: true}
	key, err := decrementKeyUsage(request.KeyName)
	if err != nil {
		slog.Error("key update", "error", err)
		errResp.Message = "key error " + err.Error()
		return errResp
	}

	if err := saveNewPeer(request.Peer); err != nil {
		slog.Debug(err.Error())
		errResp.Message = err.Error()
		return errResp
	}
	if err != addNKeyUser(request.Peer) {
		errResp.Message = err.Error()
		slog.Debug(errResp.Message)
		return errResp
	}
	for _, network := range key.Networks {
		if err := addPeerToNetwork(request.Peer, network); err != nil {
			errResp.Message = err.Error()
			slog.Debug(errResp.Message)
			return errResp
		}
	}
	return plexus.NetworkResponse{
		Message: fmt.Sprintf("joined network(s) successfully %v", key.Networks),
	}
}

func saveNewPeer(peer plexus.Peer) error {
	if _, err := boltdb.Get[plexus.Peer](peer.WGPublicKey, "peers"); err == nil {
		return errors.New("peer exists")
	}
	// save new peer(device)
	if err := boltdb.Save(peer, peer.WGPublicKey, "peers"); err != nil {
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

func addPeerToNetwork(peer plexus.Peer, network string) error {
	netToUpdate, err := boltdb.Get[plexus.Network](network, "networks")
	if err != nil {
		return err
	}
	// check if peer is already part of network
	for _, existing := range netToUpdate.Peers {
		if existing.WGPublicKey == peer.WGPublicKey {
			return fmt.Errorf("peer exists in network %s", network)
		}
	}
	addr, err := getNextIP(netToUpdate)
	if err != nil {
		return fmt.Errorf("unable to get ip for peer %s %s %v", peer.WGPublicKey, network, err)
	}
	slog.Debug("setting ip to", "ip", addr)
	update := plexus.NetworkUpdate{
		Type: plexus.AddPeer,
		Peer: plexus.NetworkPeer{
			WGPublicKey:      peer.WGPublicKey,
			HostName:         peer.Name,
			PublicListenPort: peer.PublicListenPort,
			Endpoint:         peer.Endpoint,
			Address: net.IPNet{
				IP:   addr,
				Mask: netToUpdate.Net.Mask,
			},
		},
	}
	netToUpdate.Peers = append(netToUpdate.Peers, update.Peer)
	if err := boltdb.Save(netToUpdate, netToUpdate.Name, "networks"); err != nil {
		slog.Error("save updated network", "error", err)
		return err
	}
	slog.Debug("publish network update", "network", network, "update", update)
	if err := encodedConn.Publish("networks."+network, update); err != nil {
		slog.Error("publish new peer", "error", err)
		return err
	}
	return nil
}

func getNextIP(network plexus.Network) (net.IP, error) {
	taken := make(map[string]bool)
	for _, peer := range network.Peers {
		taken[peer.Address.IP.String()] = true
	}
	slog.Debug("getnextIP", "network", network)
	slog.Debug("getNextIP", "taken", taken)
	slog.Debug("getNextIP", "net", network.Net)
	pretty.Println(network.Net)
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

// checkinHandler handle messages published to checkin.<ID>
func checkinHandler(m *nats.Msg) {
	parts := strings.Split(m.Subject, ".")
	if len(parts) < 2 {
		slog.Error("invalid topic")
		return
	}
	peerID := parts[1]
	//update, err := database.GetDevice(device)
	slog.Info("received checkin", "device", peerID)
	peer, err := boltdb.Get[plexus.Peer](peerID, "peers")
	if err != nil {
		slog.Error("peer checkin", "error", err)
		m.Respond([]byte("error " + err.Error()))
		return
	}
	peer.Updated = time.Now()
	if err := boltdb.Save(peer, peer.WGPublicKey, "peers"); err != nil {
		slog.Error("peer checkin save", "error", err)
		m.Respond([]byte("error " + err.Error()))
		return
	}
	m.Respond([]byte("ack"))
}

// configHandler handles requests for device configuration ie request published to config.<ID>
func configHandler(m *nats.Msg) {
	device := m.Subject[7:]
	slog.Info("received config request", "device", device)
	config := getConfig(device)
	if config == nil {
		m.Header.Set("error", "empty")
	}
	m.Respond(config)
}

// connectivityHandler handles connectivity stats ie message published to connectivity.<ID>
func connectivityHandler(m *nats.Msg) {
	device := m.Subject[13:]
	slog.Info("received connectivity stats", "device", device)
	data := plexus.ConnectivityData{}
	if err := json.Unmarshal(m.Data, &data); err != nil {
		m.Header.Set("error", "invalid data")
		m.Respond([]byte("nack"))
		return
	}
	network, err := boltdb.Get[plexus.Network](data.Network, "networks")
	if err != nil {
		m.Header.Set("error", "no such network")
		m.Respond([]byte("nack"))
		return
	}
	updatedPeers := []plexus.NetworkPeer{}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == device {
			peer.Connectivity = data.Connectivity
		}
		updatedPeers = append(updatedPeers, peer)
	}
	network.Peers = updatedPeers
	boltdb.Save(network, network.Name, "networks")
	m.Respond([]byte("ack"))
}

// leaveHandler handles leaving a network
func leaveHandler(m *nats.Msg) {
	device := m.Subject[6:]
	netName := string(m.Data)
	slog.Debug("leave handler", "peer", device, "network", netName)
	network, err := boltdb.Get[plexus.Network](netName, "networks")
	if err != nil {
		slog.Error("get network to leave", "error", err)
		m.Respond([]byte("error " + err.Error()))
		return
	}
	found := false
	for i, peer := range network.Peers {
		var errs error
		if peer.WGPublicKey != device {
			continue
		}
		found = true
		network.Peers = slices.Delete(network.Peers, i, i+1)
		if err := boltdb.Save(network, network.Name, "networks"); err != nil {
			errs = errors.Join(errs, err)
		}
		update := plexus.NetworkUpdate{
			Type: plexus.DeletePeer,
			Peer: peer,
		}
		payload, err := json.Marshal(&update)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		slog.Debug("publishing network update for peer leaving network", "network", netName, "peer", device)
		if err := natsConn.Publish("networks."+netName, payload); err != nil {
			errs = errors.Join(errs, err)
		}
		if errs != nil {
			m.Respond([]byte("errors " + errs.Error()))
			slog.Error("errors removing peer from network", "peer", device, "network", netName, "error(s)", errs)
		}
	}
	if !found {
		m.Respond([]byte(fmt.Sprintf("error: %s not a part of %s network", device, netName)))
		slog.Error("peer not found", "peer", device, "network", netName)
		return
	}
	m.Respond([]byte(fmt.Sprintf("%s deleted from %s network", device, netName)))
}

func processUpdate(request *plexus.NetworkRequest) plexus.NetworkResponse {
	switch request.Action {
	case plexus.ConnectToNetwork:
		return connectToNetwork(request)
	default:
		return plexus.NetworkResponse{
			Error:   true,
			Message: "invalid request action",
		}
	}
}

func connectToNetwork(request *plexus.NetworkRequest) plexus.NetworkResponse {
	errResponse := plexus.NetworkResponse{Error: true}
	_, err := boltdb.Get[plexus.Peer](request.Peer.WGPublicKey, "peers")
	if errors.Is(err, boltdb.ErrNoResults) {
		if err := saveNewPeer(request.Peer); err != nil {
			errResponse.Message = err.Error()
			return errResponse
		}
		if err := addNKeyUser(request.Peer); err != nil {
			errResponse.Message = err.Error()
			return errResponse
		}
	}
	if err := addPeerToNetwork(request.Peer, request.Network); err != nil {
		errResponse.Message = err.Error()
		return errResponse
	}
	return plexus.NetworkResponse{
		Message: fmt.Sprintf("%s added to network %s", request.Peer.WGPublicKey, request.Network),
	}
}
