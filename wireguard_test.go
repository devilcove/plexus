package plexus

import (
	"net"
	"os/user"
	"testing"
	"time"

	"github.com/Kairum-Labs/should"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestNew(t *testing.T) {
	deleteAllWireguardInterfaces(t)
	user, err := user.Current()
	should.NotBeError(t, err)
	if user.Uid != "0" {
		t.Log("this test must be run as root")
		t.Skip()
	}
	key, err := wgtypes.GeneratePrivateKey()
	should.NotBeError(t, err)
	peerKey, err := wgtypes.ParseKey("uREcerxMksoD3K0dy1ciJDRGzGCJ8jvIzJ5r9jWApXY=")
	should.NotBeError(t, err)
	key2, err := wgtypes.GeneratePrivateKey()
	should.NotBeError(t, err)
	peer2Key := key2.PublicKey()
	key3, err := wgtypes.GeneratePrivateKey()
	should.NotBeError(t, err)
	peer3Key := key3.PublicKey()
	port := 51820
	keepalive := time.Second * 25
	config := wgtypes.Config{
		PrivateKey:   &key,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:         peerKey,
				ReplaceAllowedIPs: true,
				AllowedIPs: []net.IPNet{
					{
						IP:   net.ParseIP("10.100.10.2"),
						Mask: net.CIDRMask(32, 32),
					},
				},
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("8.8.8.8"),
					Port: 51820,
				},
				PersistentKeepaliveInterval: &keepalive,
			},
			{
				PublicKey:         peer2Key,
				ReplaceAllowedIPs: true,
				AllowedIPs: []net.IPNet{
					{
						IP:   net.ParseIP("10.100.10.3"),
						Mask: net.CIDRMask(32, 32),
					},
				},
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("8.8.8.9"),
					Port: 51820,
				},
				PersistentKeepaliveInterval: &keepalive,
			},
		},
	}
	address := netlink.Addr{
		IPNet: &net.IPNet{
			IP:   net.ParseIP("10.100.10.1"),
			Mask: net.CIDRMask(24, 32),
		},
	}
	newPeer := wgtypes.PeerConfig{
		PublicKey: peer3Key,
		Endpoint: &net.UDPAddr{
			IP:   net.ParseIP("8.8.8.10"),
			Port: 51820,
		},
		ReplaceAllowedIPs: true,
		AllowedIPs: []net.IPNet{
			{
				IP:   net.ParseIP("10.100.10.4"),
				Mask: net.CIDRMask(32, 32),
			},
			{
				IP:   net.ParseIP("10.200.10.1"),
				Mask: net.CIDRMask(32, 32),
			},
		},
	}
	t.Run("new", func(t *testing.T) {
		wg := New("wgtest", 1420, address, config)
		should.BeEqual(t, wg.Attrs().Name, "wgtest")
		should.BeEqual(t, wg.Attrs().MTU, 1420)
		should.BeEqual(t, len(wg.Config.Peers), 2)
		err = wg.Up()
		should.NotBeError(t, err)
		should.BeEqual(t, wg.Type(), "wireguard")
		link, err := netlink.LinkByName(wg.Name)
		should.NotBeError(t, err)
		should.BeEqual(t, wg.Attrs().Index, link.Attrs().Index)
		routes, err := netlink.RouteGet(net.ParseIP("10.100.10.10"))
		should.NotBeError(t, err)
		should.BeEqual(t, routes[0].LinkIndex, wg.Attrs().Index)
		device, err := GetDevice("wgtest")
		should.NotBeError(t, err)
		should.BeEqual(t, device.Name, "wgtest")
		should.BeEqual(t, device.Type, wgtypes.LinuxKernel)
		should.BeEqual(t, len(device.Peers), 2)
		wg, err = Get("wgtest")
		should.NotBeError(t, err)
		should.BeEqual(t, wg.Name, "wgtest")
		should.BeEqual(t, len(wg.Config.Peers), 2)
	})
	// Peers
	t.Run("addPeer", func(t *testing.T) {
		wg, err := Get("wgtest")
		should.NotBeError(t, err)
		should.BeEqual(t, len(wg.Config.Peers), 2)
		wg.AddPeer(newPeer)
		err = wg.Apply()
		should.NotBeError(t, err)
		device, err := GetDevice("wgtest")
		should.NotBeError(t, err)
		should.BeEqual(t, len(device.Peers), 3)
	})
	t.Run("replacePeer", func(t *testing.T) {
		newPeer.Endpoint = &net.UDPAddr{
			IP:   net.ParseIP("8.8.8.20"),
			Port: 51821,
		}
		wg, err := Get("wgtest")
		should.NotBeError(t, err)
		wg.ReplacePeer(newPeer)
		err = wg.Apply()
		should.NotBeError(t, err)
		device, err := GetDevice("wgtest")
		should.NotBeError(t, err)
		should.BeEqual(t, len(device.Peers), 3)
		found := false
		for _, peer := range wg.Config.Peers {
			if peer.PublicKey == newPeer.PublicKey {
				found = true
				should.BeEqual(t, peer.Endpoint, newPeer.Endpoint)
			}
		}
		should.BeTrue(t, found)
	})
	t.Run("deletePeer", func(t *testing.T) {
		wg, err := Get("wgtest")
		should.NotBeError(t, err)
		t.Log(key2.PublicKey())
		wg.DeletePeer(key2.PublicKey().String())
		err = wg.Apply()
		should.NotBeError(t, err)
		device, err := GetDevice("wgtest")
		should.NotBeError(t, err)
		should.BeEqual(t, len(device.Peers), 2)
	})

	// shutdown
	t.Run("shutdown", func(t *testing.T) {
		wg, err := Get("wgtest")
		should.NotBeError(t, err)
		err = wg.Down()
		should.NotBeError(t, err)
	})
}

func TestGet(t *testing.T) {
	t.Run("noSuchDevice", func(t *testing.T) {
		device, err := GetDevice("doesnotexist")
		should.BeNil(t, device)
		should.BeError(t, err)
	})
	t.Run("noSuchInterface", func(t *testing.T) {
		wg, err := Get("doesnotexist")
		should.BeNil(t, wg)
		should.BeError(t, err)
	})
}

func deleteAllWireguardInterfaces(t *testing.T) {
	t.Helper()
	links, err := netlink.LinkList()
	should.NotBeError(t, err)
	for _, link := range links {
		if link.Type() == "wireguard" {
			err := netlink.LinkDel(link)
			should.NotBeError(t, err)
		}
	}
}
