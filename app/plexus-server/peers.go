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
	"github.com/kr/pretty"
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
			pretty.Println(peer.NatsConnected)
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
	if err := encodedConn.Publish(peer.WGPublicKey+".delete", plexus.DeviceUpdate{Type: 1}); err != nil {
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
