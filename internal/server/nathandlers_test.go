package server

import (
	"net"
	"testing"

	"github.com/c-robinson/iplib"
	"github.com/devilcove/plexus"
	"github.com/stretchr/testify/assert"
)

func TestGetNextIP(t *testing.T) {
	_, cidr, err := net.ParseCIDR("192.168.0.10/24")
	assert.Nil(t, err)
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
	assert.Nil(t, err)
	assert.Equal(t, 0, iplib.CompareIPs(ip, net.ParseIP("192.168.0.3")))
	t.Log(ip)
}
