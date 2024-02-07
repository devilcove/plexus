package main

import (
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"slices"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func displayAddNetwork(c *gin.Context) {
	session := sessions.Default(c)
	page := getPage(session.Get("user"))
	page.Page = "addNetwork"
	c.HTML(http.StatusOK, "addNetwork", page)

}

func addNetwork(c *gin.Context) {
	var errs error
	network := plexus.Network{}
	if err := c.Bind(&network); err != nil {
		processError(c, http.StatusBadRequest, "invalid network data")
		return
	}
	network.ServerURL = config.FQDN
	_, cidr, err := net.ParseCIDR(network.AddressString)
	if err != nil {
		log.Println("net.ParseCIDR", network.AddressString)
		processError(c, http.StatusBadRequest, "invalid address for network")
		return
	}
	network.Net = *cidr
	network.AddressString = network.Net.String()
	if !validateNetworkName(network.Name) {
		errs = errors.Join(errs, errors.New("invalid network name"))
	}
	if !validateNetworkAddress(network.Net) {
		errs = errors.Join(errs, errors.New("network address is not private"))
	}
	if errs != nil {
		processError(c, http.StatusBadRequest, errs.Error())
		return
	}
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		processError(c, http.StatusInternalServerError, "database error "+err.Error())
		return
	}
	for _, net := range networks {
		if net.Name == network.Name {
			processError(c, http.StatusBadRequest, "network name exists")
			return
		}
		if net.Net.IP.Equal(network.Net.IP) {
			processError(c, http.StatusBadRequest, "network CIDR in use by "+net.Name)
			return
		}
	}
	slog.Info("network validation complete ... saving", "network", network)
	if err := boltdb.Save(network, network.Name, "networks"); err != nil {
		processError(c, http.StatusInternalServerError, "unable to save network "+err.Error())
		return
	}
	displayNetworks(c)
}

func displayNetworks(c *gin.Context) {
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.HTML(http.StatusOK, "networks", networks)
}

func networkDetails(c *gin.Context) {
	details := struct {
		Name  string
		Peers []plexus.Peer
	}{}
	networkName := c.Param("id")
	network, err := boltdb.Get[plexus.Network](networkName, "networks")
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	for _, peer := range network.Peers {
		p, err := boltdb.Get[plexus.Peer](peer.WGPublicKey, "peers")
		if err != nil {
			slog.Error("could not obtains peer for network details", "peer", peer.WGPublicKey, "network", network, "error", err)
			continue
		}
		if time.Since(p.Updated) < time.Second*10 {
			p.NatsConnected = true
		}
		details.Peers = append(details.Peers, p)
	}
	details.Name = networkName
	c.HTML(http.StatusOK, "networkDetails", details)
}

func deleteNetwork(c *gin.Context) {
	network := c.Param("id")
	if err := boltdb.Delete[plexus.Network](network, "networks"); err != nil {
		if errors.Is(err, boltdb.ErrNoResults) {
			processError(c, http.StatusBadRequest, "network does not exist")
			return
		}
		processError(c, http.StatusInternalServerError, "delete network "+err.Error())
		return
	}
	displayNetworks(c)
}

func validateNetworkName(name string) bool {
	if len(name) > 255 {
		return false
	}
	valid := regexp.MustCompile(`^[a-z,-]+$`)
	return valid.MatchString(name)
}

func validateNetworkAddress(address net.IPNet) bool {
	return address.IP.IsPrivate()
}

func removePeerFromNetwork(c *gin.Context) {
	netName := c.Param("id")
	peerid := c.Param("peer")
	network, err := boltdb.Get[plexus.Network](netName, "networks")
	if err != nil {
		processError(c, http.StatusBadRequest, "invalid network"+err.Error())
		return
	}
	found := false
	for i, peer := range network.Peers {
		if peer.WGPublicKey == peerid {
			found = true
			slog.Info("deleting peer", "peer", peer.WGPublicKey, "network", network.Name)
			network.Peers = slices.Delete(network.Peers, i, i+1)
			if err := boltdb.Save(network, network.Name, "networks"); err != nil {
				slog.Error("save network after peer deletion", "error", err)
				processError(c, http.StatusInternalServerError, err.Error())
				return
			}
			update := plexus.NetworkUpdate{
				Type: plexus.DeletePeer,
				Peer: peer,
			}
			payload, err := json.Marshal(&update)
			if err != nil {
				slog.Error("marshal network update", "error", err)
				processError(c, http.StatusInternalServerError, err.Error())
				return
			}
			slog.Info("publishing network update", "topic", "networks."+network.Name)
			if err := natsConn.Publish("networks."+network.Name, payload); err != nil {
				slog.Error("pub delete peer", "peer", peerid, "network", netName, "error", err)
				processError(c, http.StatusInternalServerError, err.Error())
				return
			}
			break
		}
	}
	if !found {
		processError(c, http.StatusBadRequest, "invalid peer")
		return
	}
	networkDetails(c)
}

func getConfig(id string) []byte {
	response := []plexus.Network{}
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("get networks", "error", err)
		return []byte{}
	}
	for _, network := range networks {
		for _, peer := range network.Peers {
			if peer.WGPublicKey == id {
				response = append(response, network)
			}
		}
	}
	payload, err := json.Marshal(response)
	if err != nil {
		slog.Error("marshal", "error", err)
		return []byte{}
	}
	return payload
}
