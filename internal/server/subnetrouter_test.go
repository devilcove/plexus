package server

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/stretchr/testify/assert"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestSubNetInUse(t *testing.T) {
	_, public, err := generateKeys()
	assert.Nil(t, err)
	err = boltdb.Delete[plexus.Network]("plexus", networkTable)
	assert.True(t, err == nil || errors.Is(err, boltdb.ErrNoResults))
	peer := plexus.NetworkPeer{
		WGPublicKey: public.String(),
		HostName:    "peer1",
	}
	network := plexus.Network{
		Name: "plexus",
		Net: net.IPNet{
			IP:   net.ParseIP("10.10.10.0").To4(),
			Mask: net.CIDRMask(20, 32),
		},
	}
	network.Peers = append(network.Peers, peer)
	err = boltdb.Save(network, network.Name, networkTable)
	assert.Nil(t, err)
	subnet := &net.IPNet{
		IP:   net.ParseIP("192.168.1.0").To4(),
		Mask: net.CIDRMask(24, 32),
	}
	t.Run("no subnets", func(t *testing.T) {
		used, kind, name, err := subnetInUse(subnet)
		assert.Nil(t, err)
		assert.False(t, used)
		assert.Equal(t, "", kind)
		assert.Equal(t, "", name)
	})
	t.Run("no overlap", func(t *testing.T) {
		peer.SubNet = net.IPNet{
			IP:   net.ParseIP("192.168.0.0").To4(),
			Mask: net.CIDRMask(24, 32),
		}
		peer.IsSubNetRouter = true
		network.Peers = []plexus.NetworkPeer{peer}
		err = boltdb.Save(network, network.Name, networkTable)
		assert.Nil(t, err)
		used, kind, name, err := subnetInUse(subnet)
		assert.Nil(t, err)
		assert.False(t, used)
		assert.Equal(t, "", kind)
		assert.Equal(t, "", name)
	})
	t.Run("overlap subnet", func(t *testing.T) {
		peer.SubNet = net.IPNet{
			IP:   net.ParseIP("192.168.0.0").To4(),
			Mask: net.CIDRMask(20, 32),
		}
		network.Peers = []plexus.NetworkPeer{peer}
		err = boltdb.Save(network, network.Name, networkTable)
		assert.Nil(t, err)
		used, kind, name, err := subnetInUse(subnet)
		assert.Nil(t, err)
		assert.True(t, used)
		assert.Equal(t, "peer", kind)
		assert.Equal(t, "peer1", name)
	})
	t.Run("overlap network", func(t *testing.T) {
		subnet = &net.IPNet{
			IP:   net.ParseIP("10.10.11.0"),
			Mask: net.CIDRMask(24, 32),
		}
		used, kind, name, err := subnetInUse(subnet)
		assert.Nil(t, err)
		assert.True(t, used)
		assert.Equal(t, "network", kind)
		assert.Equal(t, "plexus", name)
	})
}

// generateKeys generates wgkeys that do not have a / in pubkey
func generateKeys() (*wgtypes.Key, *wgtypes.Key, error) {
	for {
		priv, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, nil, err
		}
		pub := priv.PublicKey()
		if !strings.Contains(pub.String(), "/") {
			return &priv, &pub, nil
		}
	}
}
