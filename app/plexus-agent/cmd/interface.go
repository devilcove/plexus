package cmd

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func startInterfaces(ctx context.Context, wg *sync.WaitGroup) {
	keepalive := time.Second * 25
	address := netlink.Addr{}
	defer wg.Done()
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("unable to get device info", "error", err)
		return
	}
	privKey, err := wgtypes.ParseKey(self.WGPrivateKey)
	if err != nil {
		slog.Error("unable to parse private key", "error", err)
		return
	}
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		if errors.Is(err, boltdb.ErrNoResults) {
			slog.Warn("no networks")
		} else {
			slog.Error("unable to read networks", "error", err)
			return
		}
	}
	for i, network := range networks {
		name := "plexus" + strconv.Itoa(i)
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
			continue
		}
	}
	<-ctx.Done()
}
