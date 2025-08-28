package server

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestSubnetInUse(t *testing.T) {
	_, public, err := generateKeys()
	should.BeNil(t, err)
	err = boltdb.Delete[plexus.Network]("plexus", networkTable)
	should.BeTrue(t, err == nil || errors.Is(err, boltdb.ErrNoResults))
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
	should.BeNil(t, err)
	t.Run("overlap network", func(t *testing.T) {
		subnet := &net.IPNet{
			IP:   net.ParseIP("10.10.11.0"),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.BeEqual(t, err, ErrSubnetInUse)
		should.BeEqual(t, kind, "network")
		should.BeEqual(t, name, "plexus")
	})
	t.Run("no subnets", func(t *testing.T) {
		subnet := &net.IPNet{
			IP:   net.ParseIP("192.168.100.0").To4(),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.BeNil(t, err)
		should.BeEmpty(t, kind)
		should.BeEmpty(t, name)
	})
	t.Run("no overlap", func(t *testing.T) {
		peer.Subnet = net.IPNet{
			IP:   net.ParseIP("192.168.0.0").To4(),
			Mask: net.CIDRMask(20, 32),
		}
		peer.IsSubnetRouter = true
		network.Peers = []plexus.NetworkPeer{peer}
		err = boltdb.Save(network, network.Name, networkTable)
		should.BeNil(t, err)
		subnet := &net.IPNet{
			IP:   net.ParseIP("10.10.100.0"),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.BeNil(t, err)
		should.BeEmpty(t, kind)
		should.BeEmpty(t, name)
	})
	t.Run("overlap subnet", func(t *testing.T) {
		subnet := &net.IPNet{
			IP:   net.ParseIP("192.168.1.0").To4(),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.BeEqual(t, err, ErrSubnetInUse)
		should.BeEqual(t, kind, "peer")
		should.BeEqual(t, name, "peer1")
	})
	t.Run("overlap virtual subnet", func(t *testing.T) {
		peer.IsSubnetRouter = true
		peer.UseVirtSubnet = true
		peer.VirtSubnet = net.IPNet{
			IP:   net.ParseIP("172.16.0.0").To4(),
			Mask: net.CIDRMask(20, 32),
		}
		network.Peers = []plexus.NetworkPeer{peer}
		err = boltdb.Save(network, network.Name, networkTable)
		should.BeNil(t, err)
		subnet := &net.IPNet{
			IP:   net.ParseIP("172.16.1.0"),
			Mask: net.CIDRMask(24, 32),
		}
		kind, name, err := subnetInUse(subnet)
		should.BeEqual(t, err, ErrSubnetInUse)
		should.BeEqual(t, kind, "peer")
		should.BeEqual(t, name, "peer1")
	})
}

// generateKeys generates wgkeys that do not have a / in pubkey.
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
