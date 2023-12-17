package database

import (
	"errors"
	"net"
	"testing"

	"github.com/devilcove/plexus"
	"github.com/stretchr/testify/assert"
)

func TestSaveNetwork(t *testing.T) {
	ip, cidr, err := net.ParseCIDR("10.10.10.0/24")
	assert.Nil(t, err)
	network := plexus.Network{
		Name: "net1",
		Address: net.IPNet{
			IP:   ip,
			Mask: cidr.Mask,
		},
		AddressString: "10.10.10.0/24",
	}
	err = SaveNetwork(&network)
	assert.Nil(t, err)
}

func TestGetNetwork(t *testing.T) {
	err := createNetwork()
	assert.Nil(t, err)
	t.Run("wrongName", func(t *testing.T) {
		net, err := GetNetwork("net2")
		assert.NotNil(t, err)
		assert.Equal(t, "no results found", err.Error())
		assert.Equal(t, plexus.Network{}, net)
	})
	t.Run("success", func(t *testing.T) {
		net, err := GetNetwork("net1")
		assert.Nil(t, err)
		assert.Equal(t, "10.10.10.0/24", net.AddressString)
	})
}

func TestGetAllNetworks(t *testing.T) {
	err := createNetwork()
	assert.Nil(t, err)
	t.Run("success", func(t *testing.T) {
		nets, err := GetAllNetworks()
		assert.Nil(t, err)
		assert.Equal(t, 1, len(nets))
	})
	t.Run("nonets", func(t *testing.T) {
		err := deleteAllNetworks()
		assert.Nil(t, err)
		nets, err := GetAllNetworks()
		assert.Nil(t, err)
		assert.Equal(t, 0, len(nets))
	})

}

func TestDeleteNetwork(t *testing.T) {
	err := createNetwork()
	assert.Nil(t, err)
	t.Run("networkExists", func(t *testing.T) {
		err := DeleteNetwork("net1")
		assert.Nil(t, err)
	})
	t.Run("nilNetwork", func(t *testing.T) {
		err := DeleteNetwork("net2")
		assert.NotNil(t, err)
		assert.Equal(t, "no results found", err.Error())
	})

}

func createNetwork() error {
	ip, cidr, err := net.ParseCIDR("10.10.10.0/24")
	if err != nil {
		return err
	}
	network := plexus.Network{
		Name: "net1",
		Address: net.IPNet{
			IP:   ip,
			Mask: cidr.Mask,
		},
		AddressString: "10.10.10.0/24",
	}
	return SaveNetwork(&network)
}

func deleteAllNetworks() (errs error) {
	networks, err := GetAllNetworks()
	if err != nil {
		return err
	}
	for _, net := range networks {
		if err := DeleteNetwork(net.Name); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}
