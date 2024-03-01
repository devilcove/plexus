package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats-server/v2/server"
)

func displayPeers(c *gin.Context) {
	displayPeers := []plexus.Peer{}
	peers, err := boltdb.GetAll[plexus.Peer]("peers")
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	//set Status for display
	for _, peer := range peers {
		if time.Since(peer.Updated) < connectedTime {
			peer.NatsConnected = true
		}
		displayPeers = append(displayPeers, peer)
	}
	c.HTML(http.StatusOK, "peers", displayPeers)
}

func peerDetails(c *gin.Context) {
	id := c.Param("id")
	peer, err := boltdb.Get[plexus.Peer](id, "peers")
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.HTML(http.StatusOK, "peerDetails", peer)
}

func deletePeer(c *gin.Context) {
	id := c.Param("id")
	peer, err := boltdb.Get[plexus.Peer](id, "peers")
	if err != nil {
		processError(c, http.StatusBadRequest, id+" "+err.Error())
		return
	}
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		processError(c, http.StatusInternalServerError, "get networks "+err.Error())
		return
	}
	for _, network := range networks {
		found := false
		for i, netpeer := range network.Peers {
			if netpeer.WGPublicKey == peer.WGPublicKey {
				found = true
				network.Peers = slices.Delete(network.Peers, i, i+1)
				update := plexus.NetworkUpdate{
					Type: plexus.DeletePeer,
					Peer: netpeer,
				}
				bytes, err := json.Marshal(update)
				if err != nil {
					slog.Error("marshal peer deletion", "error", err)
				}
				slog.Info("publishing network update", "type", update.Type, "network", network.Name)
				if err := natsConn.Publish("networks."+network.Name, bytes); err != nil {
					slog.Error("publish net update", "error", err)
				}
			}
		}
		if found {
			if err := boltdb.Save(network, network.Name, "networks"); err != nil {
				slog.Error("save network during peer deletion", "error", err)
			}
		}
	}
	if err := boltdb.Delete[plexus.Peer](peer.WGPublicKey, "peers"); err != nil {
		processError(c, http.StatusInternalServerError, "delete peer "+peer.Name+""+err.Error())
		return
	}
	if err := encodedConn.Publish(peer.WGPublicKey, plexus.DeviceUpdate{Type: plexus.LeaveServer}); err != nil {
		slog.Error("publish peer deletion", "error", err)
	}
	deletePeerFromBroker(peer.PubNkey)
	displayPeers(c)
}

func getDeviceUsers() []*server.NkeyUser {
	devices := []*server.NkeyUser{}
	peers, err := boltdb.GetAll[plexus.Peer]("peers")
	if err != nil {
		slog.Error("retrive peers", "error", err)
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
	peers, err := boltdb.GetAll[plexus.Peer]("peers")
	if err != nil {
		slog.Error("get peers")
		return
	}
	for _, peer := range peers {
		current := peer.NatsConnected
		if err := encodedConn.Publish(peer.WGPublicKey, plexus.UpdateRequest{Action: plexus.Ping}); err != nil {
			peer.NatsConnected = false
		} else {
			peer.NatsConnected = true
		}
		if peer.NatsConnected != current {
			savePeer(peer)
		}
	}
}

func savePeer(peer plexus.Peer) {
	slog.Debug("saving peer", "peer", peer.Name, "key", peer.WGPublicKey)
	if err := boltdb.Save(peer, peer.WGPublicKey, "peer"); err != nil {
		slog.Error("save peer", "peer", peer.Name, "error", err)
	}
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("get networks", "error", err)
	}
	for _, network := range networks {
		for i, netPeer := range network.Peers {
			if netPeer.WGPublicKey == peer.WGPublicKey {
				network.Peers[i].NatsConnected = peer.NatsConnected
				slog.Debug("saving network peer", "network", network.Name, "peer", netPeer.HostName, "key", netPeer.WGPublicKey)
				if err := boltdb.Save(network, network.Name, "networks"); err != nil {
					slog.Error("save network", "network", network.Name, "error", err)
				}
			}
		}
	}
}
