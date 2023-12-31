package main

import (
	"encoding/json"
	"log/slog"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func devicePermissions(id string) *server.Permissions {
	return &server.Permissions{
		Publish: &server.SubjectPermission{
			Allow: []string{"checkin." + id, "update." + id},
		},
		Subscribe: &server.SubjectPermission{
			Allow: []string{"network.>", "_INBOX.>"},
		},
	}
}

func joinHandler(msg *nats.Msg) {
	request := plexus.JoinRequest{}
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		slog.Error("unable to decode join data", "error", err)
		msg.Respond([]byte("unable to decode join data"))
		return
	}
	slog.Info("join request", "request", request)
	// decrement (and delete if necessary) usage remaining on key
	key, err := decrementKeyUsage(request.KeyName)
	if err != nil {
		slog.Error("key update", "error", err)
		msg.Respond([]byte("invalid key"))
		return
	}
	// save new peer(device)
	if err := boltdb.Save(request.Peer, request.Peer.WGPublicKey, "peers"); err != nil {
		slog.Error("unable to save new peer", "error", err)
		msg.Respond([]byte("failed to save peer" + err.Error()))
		return
	}
	// add new peer to networks and publish to network topic
	netPeer := plexus.NetworkPeer{
		WGPublicKey:      request.Peer.WGPublicKey,
		PublicListenPort: request.Peer.PublicListenPort,
		Endpoint:         request.Peer.Endpoint,
	}
	data, err := json.Marshal(&netPeer)
	if err != nil {
		slog.Error("marshal new network peer", "error", err)
	}
	joinResponse := []plexus.Network{}
	for _, network := range key.Networks {
		netToUpdate, err := boltdb.Get[plexus.Network](network, "networks")
		if err != nil {
			slog.Error("retrieve network", "error", err)
			continue
		}
		// check if peer is already part of network
		found := false
		for _, peer := range netToUpdate.Peers {
			if peer.WGPublicKey == netPeer.WGPublicKey {
				found = true
			}
		}
		// if not add peer to network
		if !found {
			netToUpdate.Peers = append(netToUpdate.Peers, netPeer)
			if err := boltdb.Save(netToUpdate, netToUpdate.Name, "networks"); err != nil {
				slog.Error("save updated network", "error", err)
				continue
			}
		}
		if err := natsConn.Publish("networks.newPeer."+network, data); err != nil {
			slog.Error("publish new peer", "error", err)
		}
		joinResponse = append(joinResponse, netToUpdate)
	}
	// add device to broker
	device := &server.NkeyUser{
		Nkey:        request.Peer.PubNkey,
		Permissions: devicePermissions(request.Peer.WGPublicKey),
	}
	natsOptions.Nkeys = append(natsOptions.Nkeys, device)
	if err := natServer.ReloadOptions(natsOptions); err != nil {
		slog.Error("add new device to broker", "error", err)
		msg.Respond([]byte("failed to add to broker"))
	}
	response, err := json.Marshal(joinResponse)
	if err != nil {
		slog.Error("marshal response to new peer", "error", err)
		msg.Respond([]byte("failed to create valid response"))
		return
	}
	if err := msg.Respond(response); err != nil {
		slog.Error("send response to join", "error", err)
	}

}
