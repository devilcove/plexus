package agent

import (
	"log/slog"

	"github.com/devilcove/plexus"
)

func publishDeviceUpdate(self *Device) {
	slog.Info("publish device update")
	serverEC := serverConn.Load()
	if serverEC == nil {
		slog.Error("not connected to server")
		return
	}
	if err := serverEC.Publish(self.WGPublicKey+plexus.UpdatePeer, plexus.Peer{
		WGPublicKey:   self.WGPublicKey,
		PubNkey:       self.PubNkey,
		Version:       self.Version,
		Name:          self.Name,
		OS:            self.OS,
		Endpoint:      self.Endpoint,
		NatsConnected: true,
	},
	); err != nil {
		slog.Error("publish device update", "error", err)
	}
}

func publishPeerUpdate(self *Device, network *Network) {
	slog.Info("publishing network peer update")
	me := getSelfFromPeers(self, network.Peers)
	serverEC := serverConn.Load()
	if serverEC == nil {
		slog.Error("not connected to server")
		return
	}
	if err := serverEC.Publish(self.WGPublicKey+plexus.UpdateNetworkPeer, plexus.NetworkPeer{
		WGPublicKey:      self.WGPublicKey,
		HostName:         self.Name,
		Address:          me.Address,
		ListenPort:       network.ListenPort,
		PublicListenPort: network.PublicListenPort,
		Endpoint:         self.Endpoint,
		NatsConnected:    true,
		Connectivity:     me.Connectivity,
		IsRelay:          me.IsRelay,
		IsRelayed:        me.IsRelayed,
		RelayedPeers:     me.RelayedPeers,
	},
	); err != nil {
		slog.Error("publish network peer update", "error", err)
	}
}

func getSelfFromPeers(self *Device, peers []plexus.NetworkPeer) *plexus.NetworkPeer {
	for _, peer := range peers {
		if peer.WGPublicKey == self.WGPublicKey {
			return &peer
		}
	}
	return nil
}
