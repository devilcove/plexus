package plexus

import (
	"net"
	"os/user"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestNew(t *testing.T) {
	user, err := user.Current()
	assert.Nil(t, err)
	if user.Uid != "0" {
		t.Log("this test must be run as root")
		t.FailNow()
	}
	key, err := wgtypes.GeneratePrivateKey()
	assert.Nil(t, err)
	peerKey, err := wgtypes.ParseKey("uREcerxMksoD3K0dy1ciJDRGzGCJ8jvIzJ5r9jWApXY=")
	assert.Nil(t, err)
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
						IP:   net.ParseIP("10.10.10.2"),
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
			IP:   net.ParseIP("10.10.10.1"),
			Mask: net.CIDRMask(24, 32),
		},
	}
	wg := New("wgtest", 1420, address, config)
	assert.Equal(t, "wgtest", wg.Attrs().Name)
	assert.Equal(t, 1420, wg.Attrs().MTU)
	err = wg.Up()
	assert.Nil(t, err)
	assert.Equal(t, "wireguard", wg.Type())
	link, err := netlink.LinkByName(wg.Name)
	assert.Nil(t, err)
	assert.Equal(t, link.Attrs().Index, wg.Attrs().Index)
	routes, err := netlink.RouteGet(net.ParseIP("10.10.10.10"))
	assert.Nil(t, err)
	assert.Equal(t, wg.Attrs().Index, routes[0].LinkIndex)
	err = wg.Down()
	assert.Nil(t, err)
}
