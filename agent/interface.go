package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func deleteInterface(name string) error {
	slog.Debug("deleting interface", "interface", name)
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface does not exist %w", err)
	}
	return netlink.LinkDel(link)
}

func deleteAllInterface() {
	slog.Debug("deleting all interfaces")
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("retrieve networks", "error", err)
	}
	log.Printf("%d interfaces to delete", len(networks))
	for _, network := range networks {
		if err := deleteInterface(network.Interface); err != nil {
			slog.Error("delete interface", "error", err)
		}
	}
}

// func startInterfaces(ctx context.Context, wg *sync.WaitGroup) {
func startInterface(self plexus.Device, network plexus.Network) error {
	keepalive := defaultKeepalive
	address := netlink.Addr{}
	privKey, err := wgtypes.ParseKey(self.WGPrivateKey)
	if err != nil {
		slog.Error("unable to parse private key", "error", err)
		return err
	}
	if _, err := netlink.LinkByName(network.Interface); err == nil {
		slog.Info("interface exists", "interface", network.Interface)
		return err
	}
	mtu := 1420
	peers := []wgtypes.PeerConfig{}
	for _, peer := range network.Peers {
		slog.Info("checking peer", "peer", peer.WGPublicKey, "address", peer.Address, "mask", network.Net.Mask)
		if peer.WGPublicKey == self.WGPublicKey {
			add := net.IPNet{
				IP:   peer.Address.IP,
				Mask: network.Net.Mask,
			}
			address.IPNet = &add
			continue
		}
		pubKey, err := wgtypes.ParseKey(peer.WGPublicKey)
		if err != nil {
			slog.Error("unable to parse public key", "key", peer.WGPublicKey, "error", err)
			continue
		}
		wgPeer := wgtypes.PeerConfig{
			PublicKey:         pubKey,
			ReplaceAllowedIPs: true,
			AllowedIPs: []net.IPNet{
				{
					IP:   peer.Address.IP,
					Mask: net.CIDRMask(32, 32),
				},
			},
			Endpoint: &net.UDPAddr{
				IP:   net.ParseIP(peer.Endpoint),
				Port: peer.PublicListenPort,
			},
			PersistentKeepaliveInterval: &keepalive,
		}
		peers = append(peers, wgPeer)
	}
	port, err := getFreePort(network.ListenPort)
	if err != nil {
		return err
	}
	config := wgtypes.Config{
		PrivateKey:   &privKey,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers:        peers,
	}
	link := plexus.New(network.Interface, mtu, address, config)
	pretty.Println(link)
	if err := link.Up(); err != nil {
		slog.Error("failed initializition interface", "interface", network.Interface, "error", err)
		return err
	}
	return nil
}

func addPeertoInterface(name string, peer plexus.NetworkPeer) error {
	iface, err := plexus.Get(name)
	if err != nil {
		return err
	}
	key, err := wgtypes.ParseKey(peer.WGPublicKey)
	if err != nil {
		return err
	}
	keepalive := defaultKeepalive
	iface.Config.Peers = append(iface.Config.Peers, wgtypes.PeerConfig{
		PublicKey: key,
		Endpoint: &net.UDPAddr{
			IP:   net.ParseIP(peer.Endpoint),
			Port: peer.PublicListenPort,
		},
		PersistentKeepaliveInterval: &keepalive,
		ReplaceAllowedIPs:           true,
		AllowedIPs: []net.IPNet{
			{
				IP:   peer.Address.IP,
				Mask: net.CIDRMask(32, 32),
			},
		},
	})
	return iface.Apply()
}

func deletePeerFromInterface(name string, peerToDelete plexus.NetworkPeer) error {
	iface, err := plexus.Get(name)
	if err != nil {
		return err
	}
	key, err := wgtypes.ParseKey(peerToDelete.WGPublicKey)
	if err != nil {
		return err
	}
	for i, peer := range iface.Config.Peers {
		if peer.PublicKey == key {
			iface.Config.Peers[i].Remove = true
		}
	}
	return iface.Apply()
}

func replacePeerInInterface(name string, replacement plexus.NetworkPeer) error {
	iface, err := plexus.Get(name)
	if err != nil {
		return err
	}
	key, err := wgtypes.ParseKey(replacement.WGPublicKey)
	if err != nil {
		return err
	}
	keepalive := defaultKeepalive
	newPeer := wgtypes.PeerConfig{
		PublicKey: key,
		Endpoint: &net.UDPAddr{
			IP:   net.ParseIP(replacement.Endpoint),
			Port: replacement.PublicListenPort,
		},
		PersistentKeepaliveInterval: &keepalive,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  []net.IPNet{replacement.Address},
	}
	for i, peer := range iface.Config.Peers {
		if peer.PublicKey == key {
			iface.Config.Peers[i] = newPeer
		}
	}
	return iface.Apply()
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
	return start, errors.New("no free ports")
}

func networkConnectivityStats(ctx context.Context, wg *sync.WaitGroup, self plexus.Device, network plexus.Network) {
	defer wg.Done()
	ticker := time.NewTicker(time.Minute * 1)
	publishConnectivity(self, network)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			publishConnectivity(self, network)
		}
	}
}

func publishConnectivity(self plexus.Device, network plexus.Network) {
	data := plexus.ConnectivityData{
		Network: network.Name,
	}
	client, err := wgctrl.New()
	if err != nil {
		slog.Warn("get client", "error", err)
		return
	}
	devices, err := client.Devices()
	if err != nil {
		slog.Warn("get wireguard devices", "error", err)
		return
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
			if time.Since(peer.LastHandshakeTime) < time.Minute*3 {
				goodHandShakes++
			}
		}
		data.Connectivity = goodHandShakes / float64(len(device.Peers))
		ec, ok := serverMap[network.ServerURL]
		if !ok {
			slog.Error("serverMap", "serverURL", network.ServerURL)
			continue
		}
		ec.Publish("connectivity."+self.WGPublicKey, data)
		slog.Debug("published connectivity", "data", data)
	}
}

func leaveNetwork(name string) error {
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Debug("get self", "error", err)
		return err
	}
	network, err := boltdb.Get[plexus.Network](name, "networks")
	if err != nil {
		slog.Debug("get network", "network", name, "error", err)
		return err
	}
	ec, ok := serverMap[network.ServerURL]
	if !ok {
		slog.Debug("server map missing entry")
		return errors.New("nats connection missing")
	}
	m := ""
	if err := ec.Request("leave."+self.WGPublicKey, name, m, NatsTimeout); err != nil {
		slog.Debug("nats request", "error", err)
		return err
	}
	if strings.Contains(m, "error") {
		slog.Debug("error in nats request response", "error", m)
		return errors.New(m)
	}
	return nil
}
