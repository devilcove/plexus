package cmd

import (
	"log/slog"
	"net"
	"time"

	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// func startInterfaces(ctx context.Context, wg *sync.WaitGroup) {
func startInterface(name string, self plexus.Device, network plexus.Network) error {
	keepalive := time.Second * 25
	address := netlink.Addr{}
	privKey, err := wgtypes.ParseKey(self.WGPrivateKey)
	if err != nil {
		slog.Error("unable to parse private key", "error", err)
		return err
	}
	if _, err := netlink.LinkByName(name); err == nil {
		slog.Info("interface exists", "interface", name)
		return err
	}
	mtu := 1420
	peers := []wgtypes.PeerConfig{}
	for _, peer := range network.Peers {
		slog.Info("checking peer", "peer", peer.WGPublicKey, "address", peer.Address, "mask", network.Net.Mask())
		if peer.WGPublicKey == self.WGPublicKey {
			add := net.IPNet{
				IP:   peer.Address,
				Mask: network.Net.Mask(),
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
					IP:   peer.Address,
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
	config := wgtypes.Config{
		PrivateKey:   &privKey,
		ListenPort:   &self.ListenPort,
		ReplacePeers: true,
		Peers:        peers,
	}
	link := plexus.New(name, mtu, address, config)
	pretty.Println(link)
	if err := link.Up(); err != nil {
		slog.Error("failed initializition interface", "interface", name, "error", err)
		return err
	}
	return nil
}
