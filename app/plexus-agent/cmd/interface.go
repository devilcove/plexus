package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const defaultStart = 51820

func deleteInterface(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface does not exist %w", err)
	}
	return netlink.LinkDel(link)
}

func deleteAllInterface() {
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("retrieve networks", "error", err)
	}
	for _, network := range networks {
		if err := deleteInterface(network.Interface); err != nil {
			slog.Error("delete interface", "error", err)
		}
	}
}

// func startInterfaces(ctx context.Context, wg *sync.WaitGroup) {
func startInterface(self plexus.Device, network plexus.Network) error {
	keepalive := time.Second * 25
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
	keepalive := time.Second * 20
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
	keepalive := time.Second * 20
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
		start = defaultStart
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
