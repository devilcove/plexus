package main

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-gonic/gin"
)

func displayAddRouter(c *gin.Context) {
	data := struct {
		Network string
		Router  string
	}{
		Network: c.Param("id"),
		Router:  c.Param("peer"),
	}
	slog.Debug("add router")
	c.HTML(http.StatusOK, "addRouterToNetwork", data)
}

func addRouter(c *gin.Context) {
	netID := c.Param("id")
	router := c.Param("peer")
	cidr := c.PostForm("cidr")
	nat := c.PostForm("nat")
	slog.Debug("subnet router", "network", netID, "router", router, "subnet", cidr, "use NAT", nat)
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !validateNetworkAddress(*subnet) {
		processError(c, http.StatusBadRequest, "invalid subnet: must be a private network")
		return
	}
	network, err := boltdb.Get[plexus.Network](netID, networkTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	update := plexus.NetworkUpdate{
		Action: plexus.UpdatePeer,
	}
	for i, peer := range network.Peers {
		if peer.WGPublicKey == router {
			peer.IsSubNetRouter = true
			if nat == "true" {
				peer.UseNat = true
			}
			peer.SubNet = *subnet
			network.Peers[i] = peer
			update.Peer = peer
			break
		}
	}
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := eConn.Publish("networks."+network.Name, update); err != nil {
		slog.Error("publish new relay", "error", err)
	}
	networkDetails(c)
}

func deleteRouter(c *gin.Context) {
	netID := c.Param("id")
	router := c.Param("peer")
	slog.Info("delete subnet router", "network", netID, "router", router)
	network, err := boltdb.Get[plexus.Network](netID, networkTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	update := plexus.NetworkUpdate{
		Action: plexus.UpdatePeer,
	}
	for i, peer := range network.Peers {
		if peer.WGPublicKey == router {
			peer.IsSubNetRouter = false
			peer.UseNat = false
			network.Peers[i] = peer
			update.Peer = peer
			break
		}
	}
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := eConn.Publish("networks."+network.Name, update); err != nil {
		slog.Error("publish new relay", "error", err)
	}
	networkDetails(c)
}
