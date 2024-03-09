package agent

import (
	"log/slog"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func deleteAllNetworks() {
	networks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		slog.Error("get networks", "error", err)
	}
	for _, network := range networks {
		if err := boltdb.Delete[Network](network.Name, networkTable); err != nil {
			slog.Error("delete network", "name", network.Name, "error", err)
		}
	}
}

func toAgentNetwork(in plexus.Network) Network {
	out := Network{}
	out.Name = in.Name
	out.ServerURL = in.ServerURL
	out.Net = in.Net
	out.Peers = in.Peers
	return out
}

func newNetworkPeer(self *Device) (plexus.NetworkPeer, error) {
	listenPort := checkPort(defaultWGPort)
	addr, err := getPublicAddPort(listenPort)
	if err != nil {
		return plexus.NetworkPeer{}, err
	}
	peer := plexus.NetworkPeer{
		WGPublicKey:      self.WGPublicKey,
		HostName:         self.Name,
		Endpoint:         addr.IP.String(),
		IsRelay:          false,
		IsRelayed:        false,
		RelayedPeers:     []string{},
		Connectivity:     0,
		NatsConnected:    true,
		ListenPort:       listenPort,
		PublicListenPort: addr.Port,
	}
	return peer, nil
}
