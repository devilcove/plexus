package main

import (
	"log/slog"
	"net/http"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats-server/v2/server"
)

func displayPeers(c *gin.Context) {
	peers, err := boltdb.GetAll[plexus.Peer]("peers")
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.HTML(http.StatusOK, "peers", peers)
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
