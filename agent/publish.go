package agent

import (
	"log/slog"

	"github.com/devilcove/plexus"
)

func publishDeviceUpdate(self *Device) {
	serverEC := serverConn.Load()
	if err := serverEC.Publish("update."+self.WGPublicKey, plexus.AgentRequest{
		Action: plexus.UpdatePeer,
		Peer: plexus.Peer{
			WGPublicKey: self.WGPublicKey,
			PubNkey:     self.PubNkey,
			Version:     self.Version,
			Name:        self.Name,
			OS:          self.OS,
			//ListenPort:       self.ListenPort,
			//PublicListenPort: self.PublicListenPort,
			Endpoint:      self.Endpoint,
			NatsConnected: true,
		},
	}); err != nil {
		slog.Error("publish device update", "error", err)
	}
}

func publishPeerUpdate(self *Device, network *Network) {
	me := getSelfFromPeers(self, network.Peers)
	serverEC := serverConn.Load()
	if err := serverEC.Publish("update."+self.WGPublicKey, plexus.AgentRequest{
		Action:  plexus.UpdateNetworkPeer,
		Network: network.Name,
		NetworkPeer: plexus.NetworkPeer{
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
	}); err != nil {
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
