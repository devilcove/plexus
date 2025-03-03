package server

import (
	"fmt"
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
	vcidr := c.PostForm("vcidr")
	var virtSubnet *net.IPNet
	slog.Debug("subnet router", "network", netID, "router", router, "subnet", cidr, "use NAT", nat)
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	if nat == "virt" {
		_, virtSubnet, err = net.ParseCIDR(vcidr)
		if err != nil {
			processError(c, http.StatusBadRequest, err.Error())
			return
		}
		if virtSubnet.Mask.String() != subnet.Mask.String() {
			processError(c, http.StatusBadRequest, "subnet/virtual subnet masks must be the same")
		}
		if message, err := validateSubnet(virtSubnet); err != nil {
			processError(c, http.StatusBadRequest, message)
			return
		}
	} else {
		if message, err := validateSubnet(subnet); err != nil {
			processError(c, http.StatusBadRequest, message)
			return
		}
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
			peer.IsSubnetRouter = true
			if nat == "nat" {
				peer.UseNat = true
			}
			if nat == "virt" {
				peer.UseVirtSubnet = true
				peer.VirtSubnet = *virtSubnet
			}
			peer.Subnet = *subnet
			network.Peers[i] = peer
			update.Peer = peer
			break
		}
	}
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	publishMessage(natsConn, "networks."+network.Name, update)
	publishMessage(natsConn, plexus.Update+update.Peer.WGPublicKey+plexus.AddRouter, update.Peer)
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
			peer.IsSubnetRouter = false
			peer.UseNat = false
			peer.UseVirtSubnet = false
			network.Peers[i] = peer
			update.Peer = peer
			break
		}
	}
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Debug("publish network update - delete router", "network", network.Name, "peer", update.Peer.HostName)
	publishMessage(natsConn, "networks."+network.Name, update)
	publishMessage(natsConn, plexus.Update+update.Peer.WGPublicKey+plexus.DeleteRouter, update.Peer)
	networkDetails(c)
}

func subnetInUse(subnet *net.IPNet) (string, string, error) {
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		slog.Debug("get networks", "error", err)
		return "", "", err
	}
	for _, network := range networks {
		if network.Net.Contains(subnet.IP) || subnet.Contains(network.Net.IP) {
			slog.Debug("subnet in use - network", "network", network.Name, "net", network.Net, "subnet", subnet)
			return "network", network.Name, ErrSubnetInUse
		}
		for _, peer := range network.Peers {
			if peer.IsSubnetRouter {
				if peer.UseVirtSubnet {
					if subnet.Contains(peer.VirtSubnet.IP) || peer.VirtSubnet.Contains(subnet.IP) {
						slog.Debug("virt subnet in use peer", "network", network.Name, "net", network.Net, "subnet", subnet, "peer", peer.VirtSubnet)
						return "peer", peer.HostName, ErrSubnetInUse
					}
				} else {
					if subnet.Contains(peer.Subnet.IP) || peer.Subnet.Contains(subnet.IP) {
						slog.Debug("subnet in use by peer", "network", network.Name, "net", network.Net, "subnet", subnet, "peer", peer.Subnet)
						return "peer", peer.HostName, ErrSubnetInUse
					}
				}
			}
		}
	}
	return "", "", nil
}

func validateSubnet(subnet *net.IPNet) (string, error) {
	if !validateNetworkAddress(*subnet) {
		return "invalid subnet: must be a private network", ErrInvalidSubnet
	}
	kind, name, err := subnetInUse(subnet)
	if err != nil {
		if kind == "" {
			return err.Error(), err
		}
		return fmt.Sprintf("subnet in use by %s %s", kind, name), ErrSubnetInUse
	}
	return "", nil
}
