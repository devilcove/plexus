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
	peers, err := boltdb.GetAll[plexus.Peer](peerTable)
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
	c.HTML(http.StatusOK, peerTable, displayPeers)
}

func peerDetails(c *gin.Context) {
	id := c.Param("id")
	peer, err := boltdb.Get[plexus.Peer](id, peerTable)
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.HTML(http.StatusOK, "peerDetails", peer)
}

func deletePeer(c *gin.Context) {
	id := c.Param("id")
	peer, err := discardPeer(id)
	if err != nil {
		processError(c, http.StatusBadRequest, id+" "+err.Error())
		return
	}
	deletePeerFromBroker(peer.PubNkey)
	displayPeers(c)
}

func discardPeer(id string) (plexus.Peer, error) {
	peer, err := boltdb.Get[plexus.Peer](id, peerTable)
	if err != nil {
		return peer, err
	}
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
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
				bytes, err := json.Marshal(update)
				if err != nil {
					slog.Error("marshal peer deletion", "error", err)
				}
				slog.Info("publishing network update", "type", update.Action, "network", network.Name)
				if err := natsConn.Publish("networks."+network.Name, bytes); err != nil {
					slog.Error("publish net update", "error", err)
				}
			}
		}
		if found {
			if err := boltdb.Save(network, network.Name, networkTable); err != nil {
				slog.Error("save network during peer deletion", "error", err)
			}
		}
	}
	if err := boltdb.Delete[plexus.Peer](peer.WGPublicKey, peerTable); err != nil {
		return peer, err
	}
	if err := eConn.Publish(plexus.Update+peer.WGPublicKey+plexus.LeaveServer, plexus.DeviceUpdate{Action: plexus.LeaveServer}); err != nil {
		slog.Error("publish peer deletion", "error", err)
	}
	return peer, nil
}

func getDeviceUsers() []*server.NkeyUser {
	devices := []*server.NkeyUser{}
	peers, err := boltdb.GetAll[plexus.Peer](peerTable)
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
	peers, err := boltdb.GetAll[plexus.Peer](peerTable)
	if err != nil {
		slog.Error("get peers")
		return
	}
	for _, peer := range peers {
		current := peer.NatsConnected
		pong := plexus.PingResponse{}
		slog.Debug("sending ping to peer", "peer", peer.Name, "id", peer.WGPublicKey)
		if err := eConn.Request(plexus.Update+peer.WGPublicKey+".ping", nil, &pong, natsTimeout); err != nil {
			peer.NatsConnected = false
		}
		if pong.Message == "pong" {
			peer.NatsConnected = true
		} else {
			peer.NatsConnected = false
		}
		if peer.NatsConnected != current {
			slog.Info("nats connection status changed", "peer", peer.Name, "ID", peer.WGPublicKey, "new status", peer.NatsConnected)
			savePeer(peer)
		}
	}
}

func savePeer(peer plexus.Peer) {
	slog.Debug("saving peer", "peer", peer.Name, "key", peer.WGPublicKey)
	if err := boltdb.Save(peer, peer.WGPublicKey, peerTable); err != nil {
		slog.Error("save peer", "peer", peer.Name, "error", err)
	}
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		slog.Error("get networks", "error", err)
	}
	for _, network := range networks {
		for i, netPeer := range network.Peers {
			if netPeer.WGPublicKey == peer.WGPublicKey {
				network.Peers[i].NatsConnected = peer.NatsConnected
				slog.Debug("saving network peer", "network", network.Name, "peer", netPeer.HostName, "key", netPeer.WGPublicKey)
				if err := boltdb.Save(network, network.Name, networkTable); err != nil {
					slog.Error("save network", "network", network.Name, "error", err)
				}
			}
		}
	}
}
