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
	session.Save()
	c.HTML(http.StatusOK, "addNetwork", page)

}

func addNetwork(c *gin.Context) {
	var errs error
	network := plexus.Network{}
	if err := c.Bind(&network); err != nil {
		processError(c, http.StatusBadRequest, "invalid network data")
		return
	}
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
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
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
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		processError(c, http.StatusInternalServerError, "unable to save network "+err.Error())
		return
	}
	displayNetworks(c)
}

func displayNetworks(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	page := getPage(user)
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	page.Data = networks
	session.Save()
	c.Header("HX-Trigger", "networkChange")
	c.HTML(http.StatusOK, "networks", page)
}

func networksSideBar(c *gin.Context) {
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.HTML(http.StatusOK, "sidebarNetworks", networks)
}

func getAvailablePeers(network plexus.Network) []plexus.Peer {
	taken := make(map[string]bool)
	peers := []plexus.Peer{}
	allPeers, err := boltdb.GetAll[plexus.Peer](peerTable)
	if err != nil {
		slog.Error("get peers", "error", err)
		return allPeers
	}
	for _, peer := range network.Peers {
		taken[peer.WGPublicKey] = true
	}
	for _, peer := range allPeers {
		_, ok := taken[peer.WGPublicKey]
		if !ok {
			peers = append(peers, peer)
		}
	}
	return peers
}

func networkAddPeer(c *gin.Context) {
	network := c.Param("id")
	peerID := c.Param("peer")
	slog.Debug("adding peer to network", "peer", peerID, "network", network)
	priv, pub, err := getListenPorts(peerID, network)
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := addPeerToNetwork(peerID, network, priv, pub); err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	networkDetails(c)
}

func networkDetails(c *gin.Context) {
	session := sessions.Default(c)
	details := struct {
		Name           string
		Peers          []plexus.NetworkPeer
		AvailablePeers []plexus.Peer
	}{}
	networkName := c.Param("id")
	network, err := boltdb.Get[plexus.Network](networkName, networkTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	for _, peer := range network.Peers {
		p, err := boltdb.Get[plexus.Peer](peer.WGPublicKey, peerTable)
		if err != nil {
			slog.Error("could not obtains peer for network details", "peer", peer.WGPublicKey, "network", network, "error", err)
			continue
		}
		slog.Debug("network details", "peer", peer.HostName, "connected", time.Since(p.Updated) < time.Second*10, "connectivity", peer.Connectivity)
		details.Peers = append(details.Peers, peer)
		slog.Debug("connectivity", "network", network.Name, "peer", peer.HostName, "connectivity", peer.Connectivity)
	}
	details.Name = networkName
	details.AvailablePeers = getAvailablePeers(network)
	session.Save()
	c.HTML(http.StatusOK, "networkDetails", details)
}

func deleteNetwork(c *gin.Context) {
	network := c.Param("id")
	if err := boltdb.Delete[plexus.Network](network, networkTable); err != nil {
		if errors.Is(err, boltdb.ErrNoResults) {
			processError(c, http.StatusBadRequest, "network does not exist")
			return
		}
		processError(c, http.StatusInternalServerError, "delete network "+err.Error())
		return
	}
	log.Println("deleting network", network)
	if err := eConn.Publish(plexus.Networks+network, plexus.NetworkUpdate{
		Action: plexus.DeleteNetwork,
	}); err != nil {
		slog.Error("publish delete network", "error", err)
	}
	displayNetworks(c)
}

func validateNetworkName(name string) bool {
	if len(name) > 255 {
		return false
	}
	valid := regexp.MustCompile(`^[a-z,-,0-9]+$`)
	return valid.MatchString(name)
}

func validateNetworkAddress(address net.IPNet) bool {
	return address.IP.IsPrivate()
}

func removePeerFromNetwork(c *gin.Context) {
	netName := c.Param("id")
	peerid := c.Param("peer")
	network, err := boltdb.Get[plexus.Network](netName, networkTable)
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
			if err := boltdb.Save(network, network.Name, networkTable); err != nil {
				slog.Error("save network after peer deletion", "error", err)
				processError(c, http.StatusInternalServerError, err.Error())
				return
			}
			update := plexus.NetworkUpdate{
				Action: plexus.DeletePeer,
				Peer:   peer,
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

func getNetworksForPeer(id string) ([]plexus.Network, error) {
	response := []plexus.Network{}
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		return response, err
	}
	for _, network := range networks {
		for _, peer := range network.Peers {
			if peer.WGPublicKey == id {
				response = append(response, network)
			}
		}
	}
	return response, nil
}

func displayAddRelay(c *gin.Context) {
	data := struct {
		Network        string
		Relay          plexus.NetworkPeer
		AvailablePeers []plexus.NetworkPeer
	}{}
	data.Network = c.Param("id")
	relay := c.Param("peer")
	slog.Debug("add relay", "network", data.Network, "relay", relay)
	network, err := boltdb.Get[plexus.Network](data.Network, networkTable)
	if err != nil {
		processError(c, http.StatusBadGateway, err.Error())
		return
	}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == relay {
			data.Relay = peer
			continue
		}
		if peer.IsRelay || peer.IsRelayed {
			continue
		}
		data.AvailablePeers = append(data.AvailablePeers, peer)
	}
	c.HTML(http.StatusOK, "addRelayToNetwork", data)
}

func addRelay(c *gin.Context) {
	netID := c.Param("id")
	relayID := c.Param("peer")
	relayedIDs := c.PostFormArray("relayed")
	network, err := boltdb.Get[plexus.Network](netID, networkTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
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
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Debug("publish network update - add relay", "network", network.Name, "relay", relayID)
	if err := eConn.Publish("networks."+network.Name, update); err != nil {
		slog.Error("publish new relay", "error", err)
	}
	networkDetails(c)
}

func deleteRelay(c *gin.Context) {
	netName := c.Param("id")
	peerID := c.Param("peer")
	slog.Info("delete relay", "network", netName, "relay", peerID)
	network, err := boltdb.Get[plexus.Network](netName, networkTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
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
			//already added above
			continue
		}
		if slices.Contains(peersToUnrelay, peer.WGPublicKey) {
			peer.IsRelayed = false
		}
		updatedPeers = append(updatedPeers, peer)
	}
	network.Peers = updatedPeers
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		processError(c, http.StatusBadRequest, "failed to save update network peers "+err.Error())
		return
	}
	if err := eConn.Publish(plexus.Networks+network.Name, update); err != nil {
		slog.Error("publish new relay", "error", err)
	}
	networkDetails(c)
}

func networkPeerDetails(c *gin.Context) {
	netName := c.Param("id")
	peerID := c.Param("peer")
	network, err := boltdb.Get[plexus.Network](netName, networkTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == peerID {
			c.HTML(http.StatusOK, "displayNetworkPeer", peer)
			return
		}
	}
	processError(c, http.StatusBadRequest, "peer not found")
}
