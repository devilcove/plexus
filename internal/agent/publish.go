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

// publish new listening ports to server
func publishListenPortUpdate(self *Device, network *Network) {
	slog.Info("publishing listen port update")
	serverEC := serverConn.Load()
	if serverEC == nil {
		slog.Error("not connected to server")
		return
	}
	if err := serverEC.Publish(self.WGPublicKey+plexus.UpdateListenPorts, plexus.ListenPortResponse{
		ListenPort:       network.ListenPort,
		PublicListenPort: network.PublicListenPort,
	},
	); err != nil {
		slog.Error("publish network peer update", "error", err)
	}
}

// publish network peer update to server
func publishNetworkPeerUpdate(self Device, peer *plexus.NetworkPeer) error {
	slog.Info("publishing network peer update")
	serverEC := serverConn.Load()
	if serverEC == nil {
		return ErrNotConnected
	}
	if err := serverEC.Publish(self.WGPublicKey+plexus.UpdateNetworkPeer, peer); err != nil {
		return err
	}
	return nil
}

func getSelfFromPeers(self *Device, peers []plexus.NetworkPeer) *plexus.NetworkPeer {
	for _, peer := range peers {
		if peer.WGPublicKey == self.WGPublicKey {
			return &peer
		}
	}
	return nil
}
