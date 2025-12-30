package server

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/publish"
)

func displayAddRouter(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Network string
		Router  string
	}{
		Network: r.PathValue("id"),
		Router:  r.PathValue("peer"),
	}
	slog.Debug("add router")
	if err := templates.ExecuteTemplate(w, "addRouterToNetwork", data); err != nil {
		slog.Error("execute template", "template", "addRouterToNetwork", "data", data, "error", err)
	}
}

func addRouter(w http.ResponseWriter, r *http.Request) {
	netID := r.PathValue("id")
	router := r.PathValue("peer")
	cidr := r.FormValue("cidr")
	nat := r.FormValue("nat")
	vcidr := r.FormValue("vcidr")
	var virtSubnet *net.IPNet
	slog.Debug("subnet router", "network", netID, "router", router, "subnet", cidr, "use NAT", nat)
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	if nat == "virt" {
		_, virtSubnet, err = net.ParseCIDR(vcidr)
		if err != nil {
			processError(w, http.StatusBadRequest, err.Error())
			return
		}
		if virtSubnet.Mask.String() != subnet.Mask.String() {
			processError(w, http.StatusBadRequest, "subnet/virtual subnet masks must be the same")
			return
		}
		if message, err := validateSubnet(virtSubnet); err != nil {
			processError(w, http.StatusBadRequest, message)
			return
		}
	}
	if message, err := validateSubnet(subnet); err != nil {
		processError(w, http.StatusBadRequest, message)
		return
	}
	network, err := boltdb.Get[plexus.Network](netID, networkTable)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
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
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	publish.Message(natsConn, "networks."+network.Name, update)
	publish.Message(natsConn, plexus.Update+update.Peer.WGPublicKey+plexus.AddRouter, update.Peer)
	networkDetails(w, r)
}

func deleteRouter(w http.ResponseWriter, r *http.Request) {
	netID := r.PathValue("id")
	router := r.PathValue("peer")
	slog.Info("delete subnet router", "network", netID, "router", router)
	network, err := boltdb.Get[plexus.Network](netID, networkTable)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
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
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	slog.Debug(
		"publish network update - delete router",
		"network", network.Name,
		"peer", update.Peer.HostName,
	)
	publish.Message(natsConn, "networks."+network.Name, update)
	publish.Message(
		natsConn,
		plexus.Update+update.Peer.WGPublicKey+plexus.DeleteRouter,
		update.Peer,
	)
	networkDetails(w, r)
}

func subnetInUse(subnet *net.IPNet) (string, string, error) {
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		slog.Debug("get networks", "error", err)
		return "", "", err
	}
	for _, network := range networks {
		if network.Net.Contains(subnet.IP) || subnet.Contains(network.Net.IP) {
			slog.Debug(
				"subnet in use - network",
				"network", network.Name,
				"net", network.Net,
				"subnet", subnet,
			)
			return "network", network.Name, ErrSubnetInUse
		}
		for _, peer := range network.Peers {
			if err := checkSubNetRouter(peer, network, subnet); err != nil {
				return "peer", peer.HostName, ErrSubnetInUse
			}
		}
	}
	return "", "", nil
}

func checkSubNetRouter(peer plexus.NetworkPeer, network plexus.Network, subnet *net.IPNet) error {
	if !peer.IsSubnetRouter {
		return nil
	}
	if peer.UseVirtSubnet {
		if subnet.Contains(peer.VirtSubnet.IP) || peer.VirtSubnet.Contains(subnet.IP) {
			slog.Debug("virt subnet in use peer", "network", network.Name, "net",
				network.Net, "subnet", subnet, "peer", peer.VirtSubnet)
			return ErrSubnetInUse
		}
	} else {
		if subnet.Contains(peer.Subnet.IP) || peer.Subnet.Contains(subnet.IP) {
			slog.Debug("subnet in use by peer", "network", network.Name, "net",
				network.Net, "subnet", subnet, "peer", peer.Subnet)
			return ErrSubnetInUse
		}
	}
	return nil
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
