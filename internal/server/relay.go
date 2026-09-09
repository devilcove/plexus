package server

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/publish"
)

func displayAddRelay(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Network        string
		Relay          plexus.NetworkPeer
		AvailablePeers []plexus.NetworkPeer
	}{}
	data.Network = r.PathValue("id")
	relay := r.PathValue("peer")
	slog.Debug("add relay", "network", data.Network, "relay", relay)
	network, err := store.Get[plexus.Network](data.Network, networkBucket)
	if err != nil {
		processError(w, http.StatusBadGateway, err.Error())
		return
	}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == relay {
			data.Relay = peer
			continue
		}
		if peer.IsRelay || peer.IsRelayed || peer.IsSubnetRouter {
			continue
		}
		data.AvailablePeers = append(data.AvailablePeers, peer)
	}
	render(w, "addRelayToNetwork", data)
}

func addRelay(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		processError(w, http.StatusBadRequest, err.Error())
	}
	netID := r.PathValue("id")
	relayID := r.PathValue("peer")
	relayedIDs := r.PostForm["relayed"]
	network, err := store.Get[plexus.Network](netID, networkBucket)
	slog.Debug("add relay", "network", netID, "relay", relayID, "relayed", relayedIDs)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	update := plexus.NetworkUpdate{
		Action: plexus.AddRelay,
	}
	peers := []plexus.NetworkPeer{}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == relayID {
			peer.IsRelay = true
			peer.RelayedPeers = relayedIDs
			update.Peer = peer
		}
		if slices.Contains(relayedIDs, peer.WGPublicKey) {
			peer.IsRelayed = true
		}
		peers = append(peers, peer)
	}
	network.Peers = peers
	if err := store.Save(network, network.Name, networkBucket); err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Debug("publish network update - add relay", "network", network.Name, "relay", relayID)
	publish.Message(natsConn, "networks."+network.Name, update)
	networkDetails(w, r)
}

func deleteRelay(w http.ResponseWriter, r *http.Request) {
	netName := r.PathValue("id")
	peerID := r.PathValue("peer")
	slog.Info("delete relay", "network", netName, "relay", peerID)
	network, err := store.Get[plexus.Network](netName, networkBucket)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	update := plexus.NetworkUpdate{
		Action: plexus.DeleteRelay,
	}
	peersToUnrelay := []string{}
	updatedPeers := []plexus.NetworkPeer{}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == peerID {
			// send the original peer to agents which will include the list of peers to unrelay
			update.Peer = peer
			peer.IsRelay = false
			peersToUnrelay = peer.RelayedPeers
			peer.RelayedPeers = []string{}
			updatedPeers = append(updatedPeers, peer)
			break
		}
	}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == peerID {
			// already added above.
			continue
		}
		if slices.Contains(peersToUnrelay, peer.WGPublicKey) {
			peer.IsRelayed = false
		}
		updatedPeers = append(updatedPeers, peer)
	}
	network.Peers = updatedPeers
	if err := store.Save(network, network.Name, networkBucket); err != nil {
		processError(w, http.StatusBadRequest, "failed to save update network peers "+err.Error())
		return
	}
	slog.Debug(
		"publish network update",
		"network", network.Name,
		"peer", update.Peer.HostName,
		"reason", "delete relay",
	)
	publish.Message(natsConn, plexus.Networks+network.Name, update)
	networkDetails(w, r)
}
