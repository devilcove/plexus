package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/publish"
	"github.com/nats-io/nats-server/v2/server"
)

func displayPeers(w http.ResponseWriter, _ *http.Request) {
	displayPeers := []plexus.Peer{}
	peers, err := store.GetAll[plexus.Peer](peerBucket)
	if err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// set Status for display
	for _, peer := range peers {
		if time.Since(peer.Updated) < connectedTime {
			peer.NatsConnected = true
		}
		displayPeers = append(displayPeers, peer)
	}
	render(w, "peers", displayPeers)
}

func peerDetails(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	peer, err := store.Get[plexus.Peer](id, peerBucket)
	if err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	render(w, "peerDetails", peer)
}

func deletePeer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	peer, err := discardPeer(id)
	if err != nil {
		processError(w, http.StatusBadRequest, id+" "+err.Error())
		return
	}
	deletePeerFromBroker(peer.PubNkey)
	displayPeers(w, r)
}

func discardPeer(id string) (plexus.Peer, error) {
	peer, err := store.Get[plexus.Peer](id, peerBucket)
	if err != nil {
		return peer, err
	}
	networks, err := store.GetAll[plexus.Network](networkBucket)
	if err != nil {
		return peer, err
	}
	for _, network := range networks {
		found := false
		for i, netpeer := range network.Peers {
			if netpeer.WGPublicKey == peer.WGPublicKey {
				found = true
				network.Peers = slices.Delete(network.Peers, i, i+1)
				update := plexus.NetworkUpdate{
					Action: plexus.DeletePeer,
					Peer:   netpeer,
				}
				slog.Info(
					"publishing network update",
					"type", update.Action,
					"network", network.Name,
				)
				publish.Message(natsConn, "networks."+network.Name, update)
			}
		}
		if found {
			if err := store.Save(network, network.Name, networkBucket); err != nil {
				slog.Error("save network during peer deletion", "error", err)
			}
		}
	}
	if err := store.Delete(peer.WGPublicKey, peerBucket); err != nil {
		return peer, err
	}
	request := &plexus.DeviceUpdate{
		Action: plexus.LeaveServer,
	}
	publish.Message(natsConn, plexus.Update+peer.WGPublicKey+plexus.LeaveServer, request)
	return peer, nil
}

func getDeviceUsers() []*server.NkeyUser {
	devices := []*server.NkeyUser{}
	peers, err := store.GetAll[plexus.Peer](peerBucket)
	if err != nil {
		slog.Error("retrieve peers", "error", err)
		return devices
	}
	for _, peer := range peers {
		device := server.NkeyUser{
			Nkey:        peer.PubNkey,
			Permissions: devicePermissions(peer.WGPublicKey),
		}
		devices = append(devices, &device)
	}
	return devices
}

func deletePeerFromBroker(key string) {
	for i, optionKey := range natsOptions.Nkeys {
		if optionKey.Nkey == key {
			natsOptions.Nkeys = slices.Delete(natsOptions.Nkeys, i, i+1)
			break
		}
	}
	if err := natServer.ReloadOptions(natsOptions); err != nil {
		slog.Error("delete peer from broker", "error", err)
	}
}

func pingPeers() {
	peers, err := store.GetAll[plexus.Peer](peerBucket)
	if err != nil {
		slog.Error("get peers")
		return
	}
	for _, peer := range peers {
		current := peer.NatsConnected
		pong := &plexus.PingResponse{}
		slog.Debug("sending ping to peer", "peer", peer.Name, "id", peer.WGPublicKey)
		msg, err := natsConn.Request(plexus.Update+peer.WGPublicKey+".ping", nil, natsTimeout)
		if err != nil {
			peer.NatsConnected = false
		} else {
			err = json.Unmarshal(msg.Data, pong)
			if err != nil {
				slog.Error("invalid ping response", "error", err)
			}
			if pong.Message == "pong" {
				peer.NatsConnected = true
			} else {
				peer.NatsConnected = false
			}
		}
		if peer.NatsConnected != current {
			slog.Info("nats connection status changed", "peer", peer.Name, "ID", peer.WGPublicKey,
				"new status", peer.NatsConnected)
			savePeer(peer)
		}
	}
}

func savePeer(peer plexus.Peer) {
	slog.Debug("saving peer", "peer", peer.Name, "key", peer.WGPublicKey)
	if err := store.Save(peer, peer.WGPublicKey, peerBucket); err != nil {
		slog.Error("save peer", "peer", peer.Name, "error", err)
	}
	networks, err := store.GetAll[plexus.Network](networkBucket)
	if err != nil {
		slog.Error("get networks", "error", err)
	}
	for _, network := range networks {
		for i, netPeer := range network.Peers {
			if netPeer.WGPublicKey == peer.WGPublicKey {
				network.Peers[i].NatsConnected = peer.NatsConnected
				slog.Debug(
					"saving network peer",
					"network", network.Name,
					"peer", netPeer.HostName,
					"key", netPeer.WGPublicKey,
				)
				if err := store.Save(network, network.Name, networkBucket); err != nil {
					slog.Error("save network", "network", network.Name, "error", err)
				}
			}
		}
	}
}
