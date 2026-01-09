package server

import (
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
	"github.com/devilcove/plexus/internal/publish"
)

func displayAddNetwork(w http.ResponseWriter, r *http.Request) {
	session := GetSessionData(r)
	page := getPage(session.Username)
	page.Page = "addNetwork"

	if err := templates.ExecuteTemplate(w, "addNetwork", page); err != nil {
		slog.Error("template", "name", "addnetwork", "error", err)
	}
}

func addNetwork(w http.ResponseWriter, r *http.Request) {
	var errs error
	network := plexus.Network{
		Name:          r.FormValue("name"),
		AddressString: r.FormValue("addressstring"),
	}
	_, cidr, err := net.ParseCIDR(network.AddressString)
	if err != nil {
		log.Println("net.ParseCIDR", network.AddressString)
		processError(w, http.StatusBadRequest, "invalid address for network")
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
		processError(w, http.StatusBadRequest, errs.Error())
		return
	}
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		processError(w, http.StatusInternalServerError, "database error "+err.Error())
		return
	}
	for _, net := range networks {
		if net.Name == network.Name {
			processError(w, http.StatusBadRequest, "network name exists")
			return
		}
		if net.Net.IP.Equal(network.Net.IP) {
			processError(w, http.StatusBadRequest, "network CIDR in use by "+net.Name)
			return
		}
	}
	slog.Debug("network validation complete ... saving", "network", network)
	if err := boltdb.Save(network, network.Name, networkTable); err != nil {
		processError(w, http.StatusInternalServerError, "unable to save network "+err.Error())
		return
	}
	displayNetworks(w, r)
}

func displayNetworks(w http.ResponseWriter, r *http.Request) {
	session := GetSessionData(r)
	page := getPage(session.Username)
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	page.Data = networks

	w.Header().Add("Hx-Trigger", "networkChange")
	if err := templates.ExecuteTemplate(w, "networks", page); err != nil {
		slog.Error("template", "name", "networks", "page", page, "error", err)
	}
}

func networksSideBar(w http.ResponseWriter, _ *http.Request) {
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := templates.ExecuteTemplate(w, "sidebarNetworks", networks); err != nil {
		slog.Error("sidebar", "networks", networks, "error", err)
	}
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

func networkAddPeer(w http.ResponseWriter, r *http.Request) {
	network := r.PathValue("id")
	peerID := r.PathValue("peer")
	slog.Debug("adding peer to network", "peer", peerID, "network", network)
	priv, pub, err := getListenPorts(peerID, network)
	if err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err := addPeerToNetwork(peerID, network, priv, pub); err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	networkDetails(w, r)
}

func networkDetails(w http.ResponseWriter, r *http.Request) {
	details := struct {
		Name           string
		Peers          []plexus.NetworkPeer
		AvailablePeers []plexus.Peer
	}{}
	networkName := r.PathValue("id")
	network, err := boltdb.Get[plexus.Network](networkName, networkTable)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, peer := range network.Peers {
		p, err := boltdb.Get[plexus.Peer](peer.WGPublicKey, peerTable)
		if err != nil {
			slog.Error(
				"could not obtains peer for network details",
				"peer", peer.WGPublicKey,
				"network", network,
				"error", err,
			)
			continue
		}
		slog.Debug("network details", "peer", peer.HostName, "connected",
			time.Since(p.Updated) < time.Second*10, "connectivity", peer.Connectivity)
		details.Peers = append(details.Peers, peer)
		slog.Debug(
			"connectivity",
			"network", network.Name,
			"peer", peer.HostName,
			"connectivity", peer.Connectivity,
		)
	}
	details.Name = networkName
	details.AvailablePeers = getAvailablePeers(network)
	if err := templates.ExecuteTemplate(w, "networkDetails", details); err != nil {
		slog.Error("template", "template", "networkDetails", "details", details, "error", err)
	}
}

func deleteNetwork(w http.ResponseWriter, r *http.Request) {
	network := r.PathValue("id")
	if err := boltdb.Delete[plexus.Network](network, networkTable); err != nil {
		if errors.Is(err, boltdb.ErrNoResults) {
			processError(w, http.StatusBadRequest, "network does not exist")
			return
		}
		processError(w, http.StatusInternalServerError, "delete network "+err.Error())
		return
	}
	log.Println("deleting network", network)
	if natsConn == nil {
		slog.Error("not connected to nats")
		processError(
			w,
			http.StatusInternalServerError,
			"nats failure:  network update not published",
		)
		return
	}
	slog.Debug("publish network update", "network", network, "reason", "delete network")
	publish.Message(
		natsConn,
		plexus.Networks+network,
		plexus.NetworkUpdate{Action: plexus.DeleteNetwork},
	)
	displayNetworks(w, r)
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

func removePeerFromNetwork(w http.ResponseWriter, r *http.Request) {
	netName := r.PathValue("id")
	peerid := r.PathValue("peer")
	network, err := boltdb.Get[plexus.Network](netName, networkTable)
	if err != nil {
		processError(w, http.StatusBadRequest, "invalid network"+err.Error())
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
				processError(w, http.StatusInternalServerError, err.Error())
				return
			}
			update := plexus.NetworkUpdate{
				Action: plexus.DeletePeer,
				Peer:   peer,
			}
			slog.Info("publishing network update", "topic", "networks."+network.Name)
			publish.Message(natsConn, "networks."+network.Name, update)
			break
		}
	}
	if !found {
		processError(w, http.StatusBadRequest, "invalid peer")
		return
	}
	networkDetails(w, r)
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

func networkPeerDetails(w http.ResponseWriter, r *http.Request) {
	netName := r.PathValue("id")
	peerID := r.PathValue("peer")
	network, err := boltdb.Get[plexus.Network](netName, networkTable)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == peerID {
			peer.Connectivity *= 100
			if err := templates.ExecuteTemplate(w, "displayNetworkPeer", peer); err != nil {
				slog.Error(
					"template execute",
					"template", "displayNetworkPeer",
					"peer", peer,
					"error", err,
				)
			}
			return
		}
	}
	processError(w, http.StatusBadRequest, "peer not found")
}
