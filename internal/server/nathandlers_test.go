package server

import (
	"net"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/c-robinson/iplib"
	"github.com/devilcove/plexus"
)

func TestGetNextIP(t *testing.T) {
	_, cidr, err := net.ParseCIDR("192.168.0.10/24")
	should.BeNil(t, err)
	network := plexus.Network{
		Net: *cidr,
	}
	peers := []plexus.NetworkPeer{
		{
			Address: net.IPNet{
				IP:   net.ParseIP("192.168.0.1"),
				Mask: network.Net.Mask,
			},
		},
		{
			Address: net.IPNet{
				IP:   net.ParseIP("192.168.0.2"),
				Mask: network.Net.Mask,
			},
		},
		{
			Address: net.IPNet{
				IP:   net.ParseIP("192.168.0.4"),
				Mask: network.Net.Mask,
			},
		},
	}
	network.Peers = peers
	ip, err := getNextIP(network)
	should.BeNil(t, err)
	should.BeEqual(t, iplib.CompareIPs(ip, net.ParseIP("192.168.0.3")), 0)
	t.Log(ip)
}
