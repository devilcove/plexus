package main

import (
	"net/http"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-gonic/gin"
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
