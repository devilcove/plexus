package agent

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"slices"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func deleteInterface(name string) error {
	slog.Info("deleting interface", "interface", name)
	defer log.Println("delete interface done")
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface does not exist %w", err)
	}
	log.Println(link.Attrs().Name, link.Attrs().Index)
	return netlink.LinkDel(link)
}

func deleteAllInterfaces() {
	slog.Debug("deleting all interfaces")
	networks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		slog.Error("retrieve networks", "error", err)
		return
	}
	log.Printf("%d interfaces to delete", len(networks))
	for _, network := range networks {
		log.Println("calling deleteInterface", network.Interface)
		if err := deleteInterface(network.Interface); err != nil {
			slog.Error("delete interface", "error", err)
			return
		}
	}
}

func startAllInterfaces(self Device) {
	networks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		slog.Error("get networks", "error", err)
		return
	}
	for _, network := range networks {
		slog.Debug("starting interface", "interface", network.Interface, "network", network.Name)
		if err := startInterface(self, network); err != nil {
			slog.Error("start interface", "network", network.Name, "interface", network.Interface, "error", err)
		}
	}
}

// func startInterfaces(ctx context.Context, wg *sync.WaitGroup) {
func startInterface(self Device, network Network) error {
	slog.Info("starting interface", "interface", network.Interface, "network", network.Name)
	address := netlink.Addr{}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == self.WGPublicKey {
			add := net.IPNet{
				IP:   peer.Address.IP,
				Mask: network.Net.Mask,
			}
			address.IPNet = &add
			break
		}
	}
	if address.IPNet == nil {
		return errors.New("no address for network" + network.Name)
	}
	privKey, err := wgtypes.ParseKey(self.WGPrivateKey)
	if err != nil {
		slog.Error("unable to parse private key", "error", err)
		return err
	}
	if _, err := netlink.LinkByName(network.Interface); err == nil {
		slog.Warn("interface exists", "interface", network.Interface)
		wg, _ := plexus.Get(network.Interface)
		wg.Apply()
		return err
	}
	mtu := 1420
	peers := getWGPeers(self, network)
	port, err := getFreePort(network.ListenPort)
	if err != nil {
		return err
	}
	addressChanged, portChanged, err := stunCheck(&self, &network, port)
	if err != nil {
		slog.Error("stun error", "error", err)
	}
	if port != network.ListenPort {
		portChanged = true
	}
	if addressChanged {
		if err := boltdb.Save(self, "self", deviceTable); err != nil {
			return err
		}
		go publishDeviceUpdate(&self)
	}
	if portChanged {
		if err := boltdb.Save(network, network.Name, networkTable); err != nil {
			return err
		}
		go publishPeerUpdate(&self, &network)
	}
	config := wgtypes.Config{
		PrivateKey:   &privKey,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers:        peers,
	}
	slog.Debug("creating new wireguard interface", "name", network.Interface, "address", address, "key", config.PrivateKey, "port", config.ListenPort)
	wg := plexus.New(network.Interface, mtu, address, config)
	if err := wg.Up(); err != nil {
		slog.Error("failed initializition interface", "interface", network.Interface, "error", err)
		return err
	}
	return nil
}

func resetPeersOnNetworkInterface(self Device, network Network) error {
	slog.Info("resetting peers", "interface", network.Interface, "network", network.Name)
	iface, err := plexus.Get(network.Interface)
	if err != nil {
		return err
	}
	iface.Config.ReplacePeers = true
	iface.Config.Peers = getWGPeers(self, network)
	if err := iface.Apply(); err != nil {
		return err
	}
	return nil
}

func getFreePort(start int) (int, error) {
	addr := net.UDPAddr{}
	if start == 0 {
		start = defaultWGPort
	}
	for x := start; x <= 65535; x++ {
		addr.Port = x
		conn, err := net.ListenUDP("udp", &addr)
		if err != nil {
			continue
		}
		conn.Close()
		return x, nil
	}
	return 0, errors.New("no free ports")
}

func getConnectivity() []plexus.ConnectivityData {
	results := []plexus.ConnectivityData{}
	networks, err := boltdb.GetAll[Network](networkTable)
	if err != nil {
		slog.Error("get networks", "error", err)
		return results
	}
	client, err := wgctrl.New()
	if err != nil {
		slog.Warn("get client", "error", err)
		return results
	}
	devices, err := client.Devices()
	if err != nil {
		slog.Warn("get wireguard devices", "error", err)
		return results
	}
	for _, network := range networks {
		data := plexus.ConnectivityData{
			Network: network.Name,
		}
		for _, device := range devices {
			if device.Name != network.Interface {
				continue
			}
			if len(device.Peers) == 0 {
				continue
			}
			goodHandShakes := 0.0
			for _, peer := range device.Peers {
				if time.Since(peer.LastHandshakeTime) < connectivityTimeout {
					goodHandShakes++
				}
			}
			data.Connectivity = goodHandShakes / float64(len(device.Peers))
		}
		results = append(results, data)
	}
	return results
}

func getAllowedIPs(node plexus.NetworkPeer, peers []plexus.NetworkPeer) []net.IPNet {
	allowed := []net.IPNet{}
	allowed = append(allowed, net.IPNet{
		IP:   node.Address.IP,
		Mask: net.CIDRMask(32, 32),
	})
	if node.IsSubNetRouter {
		allowed = append(allowed, node.SubNet)
	}
	if node.IsRelay {
		for _, peer := range peers {
			if peer.IsRelayed {
				if slices.Contains(node.RelayedPeers, peer.WGPublicKey) {
					allowed = append(allowed, peer.Address)
				}
			}
		}
	}
	return allowed
}

func getWGPeers(self Device, network Network) []wgtypes.PeerConfig {
	keepalive := defaultKeepalive
	peers := []wgtypes.PeerConfig{}
	for _, peer := range network.Peers {
		slog.Debug("checking peer", "peer", peer.WGPublicKey, "address", peer.Address, "mask", network.Net.Mask)
		if peer.WGPublicKey == self.WGPublicKey {
			if peer.IsRelayed {
				slog.Info("I am relayed")
				return selfRelayedPeers(self, network)
			}
			if peer.IsRelay {
				slog.Info("I am a relay")
				//turn off relayed status
				for i := range network.Peers {
					if slices.Contains(peer.RelayedPeers, network.Peers[i].WGPublicKey) {
						network.Peers[i].IsRelayed = false
					}
				}
			}
		}
	}
	for _, peer := range network.Peers {
		if peer.WGPublicKey == self.WGPublicKey {
			slog.Debug("skipping self")
			continue
		}
		if peer.IsRelayed {
			slog.Debug("skipping relayed peer", "peer", peer.HostName)
			continue
		}
		slog.Debug("adding peer", "peer", peer.HostName, "key", peer.WGPublicKey)
		pubKey, err := wgtypes.ParseKey(peer.WGPublicKey)
		if err != nil {
			slog.Error("unable to parse public key", "key", peer.WGPublicKey, "error", err)
			continue
		}
		wgPeer := wgtypes.PeerConfig{
			PublicKey:         pubKey,
			ReplaceAllowedIPs: true,
			AllowedIPs:        getAllowedIPs(peer, network.Peers),
			Endpoint: &net.UDPAddr{
				IP:   peer.Endpoint,
				Port: peer.PublicListenPort,
			},
			PersistentKeepaliveInterval: &keepalive,
		}
		peers = append(peers, wgPeer)
	}
	return peers
}

func selfRelayedPeers(self Device, network Network) []wgtypes.PeerConfig {
	keepalive := defaultKeepalive
	for _, peer := range network.Peers {
		if slices.Contains(peer.RelayedPeers, self.WGPublicKey) {
			pubKey, err := wgtypes.ParseKey(peer.WGPublicKey)
			if err != nil {
				slog.Error("unable to parse public key", "key", peer.WGPublicKey, "error", err)
				continue
			}
			wgPeer := wgtypes.PeerConfig{
				PublicKey:         pubKey,
				ReplaceAllowedIPs: true,
				AllowedIPs:        []net.IPNet{network.Net},
				Endpoint: &net.UDPAddr{
					IP:   peer.Endpoint,
					Port: peer.PublicListenPort,
				},
				PersistentKeepaliveInterval: &keepalive,
			}
			return []wgtypes.PeerConfig{wgPeer}
		}
	}
	slog.Error("relay not found for self")
	return []wgtypes.PeerConfig{}
}

func stunCheck(self *Device, network *Network, port int) (bool, bool, error) {
	endpointChanged := false
	portChanged := false
	stunAddr, err := getPublicAddPort(port)
	if err != nil {
		return endpointChanged, portChanged, err
	}
	if !stunAddr.IP.Equal(self.Endpoint) {
		endpointChanged = true
		self.Endpoint = stunAddr.IP
	}
	if network.PublicListenPort != stunAddr.Port {
		network.PublicListenPort = stunAddr.Port
		portChanged = true
	}
	return endpointChanged, portChanged, nil
}

func getNewListenPorts(name string) (plexus.NetworkPeer, error) {
	network := Network{}
	network.Name = name
	port, err := getFreePort(defaultWGPort)
	if err != nil {
		return plexus.NetworkPeer{}, err
	}
	self, err := boltdb.Get[Device]("self", deviceTable)
	if err != nil {
		return plexus.NetworkPeer{}, err
	}
	endpointChanged, _, err := stunCheck(&self, &network, port)
	if err != nil {
		return plexus.NetworkPeer{}, err
	}
	if endpointChanged {
		go func() {
			publishDeviceUpdate(&self)
			if err := boltdb.Save(self, "self", deviceTable); err != nil {
				slog.Error("save device", "error", err)
			}
		}()
	}
	return plexus.NetworkPeer{
		WGPublicKey:      self.WGPublicKey,
		HostName:         self.Name,
		ListenPort:       port,
		PublicListenPort: network.PublicListenPort,
	}, nil
}

func convertPeerToWG(netPeer plexus.NetworkPeer, peers []plexus.NetworkPeer) (wgtypes.PeerConfig, error) {
	keepalive := defaultKeepalive
	key, err := wgtypes.ParseKey(netPeer.WGPublicKey)
	if err != nil {
		return wgtypes.PeerConfig{}, err
	}
	return wgtypes.PeerConfig{
		PublicKey:                   key,
		PersistentKeepaliveInterval: &keepalive,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  getAllowedIPs(netPeer, peers),
	}, nil
}
