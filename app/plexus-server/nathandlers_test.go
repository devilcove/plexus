package main

import (
	"net"
	"testing"

	"github.com/c-robinson/iplib"
	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

func TestGetNextIP(t *testing.T) {

	network := plexus.Network{
		Net: iplib.Net4FromStr("192.168.0.10/24"),
	}
	peers := []plexus.NetworkPeer{
		{
			Address: net.IPNet{
				IP:   network.Net.FirstAddress(),
				Mask: network.Net.Mask(),
			},
		},
		{
			Address: net.IPNet{
				IP:   net.ParseIP("192.168.0.2"),
				Mask: network.Net.Mask(),
			},
		},
		{
			Address: net.IPNet{
				IP:   net.ParseIP("192.168.0.4"),
				Mask: network.Net.Mask(),
			},
		},
	}
	network.Peers = peers
	t.Log(pretty.Println(network))
	ip, err := getNextIP(network)
	assert.Nil(t, err)
	assert.Equal(t, 0, iplib.CompareIPs(ip, net.ParseIP("192.168.0.3")))
	t.Log(ip)
}
