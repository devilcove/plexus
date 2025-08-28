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
	user, err := user.Current()
	should.BeNil(t, err)
	if user.Uid != "0" {
		t.Log("this test must be run as root")
		t.Skip()
	}
	key, err := wgtypes.GeneratePrivateKey()
	should.BeNil(t, err)
	peerKey, err := wgtypes.ParseKey("uREcerxMksoD3K0dy1ciJDRGzGCJ8jvIzJ5r9jWApXY=")
	should.BeNil(t, err)
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
		},
	}
	address := netlink.Addr{
		IPNet: &net.IPNet{
			IP:   net.ParseIP("10.100.10.1"),
			Mask: net.CIDRMask(24, 32),
		},
	}
	wg := New("wgtest", 1420, address, config)
	should.BeEqual(t, wg.Attrs().Name, "wgtest")
	should.BeEqual(t, wg.Attrs().MTU, 1420)
	err = wg.Up()
	should.BeNil(t, err)
	should.BeEqual(t, wg.Type(), "wireguard")
	link, err := netlink.LinkByName(wg.Name)
	should.BeNil(t, err)
	should.BeEqual(t, wg.Attrs().Index, link.Attrs().Index)
	routes, err := netlink.RouteGet(net.ParseIP("10.100.10.10"))
	should.BeNil(t, err)
	should.BeEqual(t, routes[0].LinkIndex, wg.Attrs().Index)
	err = wg.Down()
	should.BeNil(t, err)
}
