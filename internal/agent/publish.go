package agent

import (
	"encoding/json"
	"log"
	"log/slog"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func publishDeviceUpdate(self *Device) {
	slog.Info("publish device update")
	serverConn := serverConn.Load()
	if serverConn == nil {
		slog.Error("not connected to server")
		return
	}
	data, err := json.Marshal(plexus.Peer{
		WGPublicKey:   self.WGPublicKey,
		PubNkey:       self.PubNkey,
		Version:       self.Version,
		Name:          self.Name,
		OS:            self.OS,
		Endpoint:      self.Endpoint,
		NatsConnected: true,
	})
	if err != nil {
		slog.Error("publish device update endcoding error", "error", err)
		return
	}
	if err := serverConn.Publish(self.WGPublicKey+plexus.UpdatePeer, data); err != nil {
		slog.Error("publish device update", "error", err)
	}
}

// publish new listening ports to server.
func publishListenPortUpdate(self *Device, network *Network) {
	slog.Info("publishing listen port update")
	natsConn := serverConn.Load()
	if natsConn == nil {
		slog.Error("not connected to server")
		return
	}
	data, err := json.Marshal(plexus.ListenPortResponse{
		ListenPort:       network.ListenPort,
		PublicListenPort: network.PublicListenPort,
	})
	if err != nil {
		slog.Error("publish listenport update endcoding error", "error", err)
		return
	}
	if err := natsConn.Publish(self.WGPublicKey+plexus.UpdateListenPorts, data); err != nil {
		slog.Error("publish listenport update", "error", err)
	}
}

// publish network peer update to server.
func publishNetworkPeerUpdate(self Device, peer *plexus.NetworkPeer) error {
	slog.Info("publishing network peer update")
	natsConn := serverConn.Load()
	if natsConn == nil {
		return ErrNotConnected
	}
	data, err := json.Marshal(peer)
	if err != nil {
		return err
	}
	if err := natsConn.Publish(self.WGPublicKey+plexus.UpdateNetworkPeer, data); err != nil {
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
				checkinData.PrivateEndpoints = append(
					checkinData.PrivateEndpoints,
					plexus.PrivateEndpoint{
						IP:      peer.PrivateEndpoint.String(),
						Network: network.Name,
					},
				)
			}
		}
	}
	serverConn := serverConn.Load()
	if serverConn == nil {
		slog.Debug("not connected to server broker .... skipping checkin")
		return
	}
	if !serverConn.IsConnected() {
		slog.Debug("not connected to server broker .... skipping checkin")
		return
	}
	checkinData.Connections = getConnectivity()
	if err := Request(serverConn, self.WGPublicKey+".checkin", checkinData, &serverResponse, NatsTimeout); err != nil {
		slog.Error("error publishing checkin ", "error", err)
		return
	}
	log.Println("checkin response from server", serverResponse.Message)
}
