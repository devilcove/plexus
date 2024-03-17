package agent

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

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

func saveServerNetworks(networks []plexus.Network) error {
	takenInterfaces := []int{}
	var err error
	for _, serverNet := range networks {
		network := toAgentNetwork(serverNet)
		network.ListenPort, err = getFreePort(defaultWGPort)
		if err != nil {
			return fmt.Errorf("unable to get freeport %w", err)
		}
		interfaceFound := false
		for i := range maxNetworks {
			if !slices.Contains(takenInterfaces, i) {
				network.InterfaceSuffix = i
				network.Interface = "plexus" + strconv.Itoa(i)
				takenInterfaces = append(takenInterfaces, i)
				interfaceFound = true
				break
			}
		}
		if !interfaceFound {
			return errors.New("no networks available")
		}
		slog.Debug("saving network", "network", network.Name)
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			return err
		}
	}
	return nil
}

func saveServerNetwork(serverNet plexus.Network) (Network, error) {
	existingNetworks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		return Network{}, err
	}
	takenInterfaces := []int{}
	for _, existing := range existingNetworks {
		takenInterfaces = append(takenInterfaces, existing.InterfaceSuffix)
	}
	slog.Debug("taken interfaces", "taken", takenInterfaces)
	network := toAgentNetwork(serverNet)
	network.ListenPort, err = getFreePort(defaultWGPort)
	if err != nil {
		return Network{}, err
	}
	for i := range maxNetworks {
		if !slices.Contains(takenInterfaces, i) {
			network.InterfaceSuffix = i
			network.Interface = "plexus" + strconv.Itoa(i)
			break
		}
	}
	slog.Debug("saving network", "network", network.Name)
	err = boltdb.Save(network, network.Name, networkTable)
	return network, err
}
