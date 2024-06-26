package agent

import (
	"log"
	"log/slog"

	"github.com/devilcove/boltdb"
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

func checkin() {
	slog.Debug("checkin")
	checkinData := plexus.CheckinData{}
	serverResponse := plexus.MessageResponse{}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		slog.Error("get device", "error", err)
		return
	}
	checkinData.ID = self.WGPublicKey
	checkinData.Version = self.Version
	checkinData.Endpoint = self.Endpoint
	networks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		slog.Error("get networks", "error", err)
		return
	}
	for _, network := range networks {
		for _, peer := range network.Peers {
			if peer.WGPublicKey != self.WGPublicKey {
				continue
			}
			if peer.PrivateEndpoint != nil {
				checkinData.PrivateEndpoints = append(checkinData.PrivateEndpoints, plexus.PrivateEndpoint{
					IP:      peer.PrivateEndpoint.String(),
					Network: network.Name,
				})
			}
		}
	}
	serverEC := serverConn.Load()
	if serverEC == nil {
		slog.Debug("not connected to server broker .... skipping checkin")
		return
	}
	if !serverEC.Conn.IsConnected() {
		slog.Debug("not connected to server broker .... skipping checkin")
		return
	}
	checkinData.Connections = getConnectivity()
	if err := serverEC.Request(self.WGPublicKey+".checkin", checkinData, &serverResponse, NatsTimeout); err != nil {
		slog.Error("error publishing checkin ", "error", err)
		return
	}
	log.Println("checkin response from server", serverResponse.Message)
}
