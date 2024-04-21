package agent

import (
	"net"
	"os/user"
	"testing"

	"github.com/devilcove/plexus"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/stretchr/testify/assert"
)

func TestAddNAT(t *testing.T) {
	table := &nftables.Table{}
	chain := &nftables.Chain{}
	user, err := user.Current()
	assert.Nil(t, err)
	if user.Uid != "0" {
		t.Log("this test must be run as root")
		t.FailNow()
	}
	c := nftables.Conn{}
	err = addNat()
	assert.Nil(t, err)
	tables, err := c.ListTables()
	assert.Nil(t, err)
	tableFound := false
	for _, t := range tables {
		if t.Name == "plexus" {
			tableFound = true
			table = t
		}
	}
	assert.True(t, tableFound)
	chains, err := c.ListChains()
	assert.Nil(t, err)
	chainFound := false
	for _, c := range chains {
		if c.Name == "plexus-nat" {
			chainFound = true
			chain = c
		}
	}
	assert.True(t, chainFound)
	rules, err := c.GetRules(table, chain)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(rules))
	assert.Equal(t, &expr.Masq{
		Random:      false,
		FullyRandom: false,
		Persistent:  false,
		ToPorts:     false,
		RegProtoMin: 0,
		RegProtoMax: 0,
	}, rules[0].Exprs[0])
	cleanNat(t, &c)
}

func TestDelNat(t *testing.T) {
	user, err := user.Current()
	assert.Nil(t, err)
	if user.Uid != "0" {
		t.Log("this test must be run as root")
		t.FailNow()
	}
	c := &nftables.Conn{}
	table := c.AddTable(&nftables.Table{
		Name:   "plexus",
		Family: nftables.TableFamilyIPv4,
	})
	chain := c.AddChain(&nftables.Chain{
		Name:     "plexus-nat",
		Table:    table,
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: nftables.ChainPriorityNATSource,
	})
	rule := &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Masq{},
		},
	}
	c.AddRule(rule)
	err = c.Flush()
	assert.Nil(t, err)
	err = delNat()
	assert.Nil(t, err)
	chains, err := c.ListChains()
	assert.Nil(t, err)
	found := false
	for _, chain := range chains {
		if chain.Name == "plexus-nat" {
			found = true
		}
	}
	assert.False(t, found)
	cleanNat(t, c)

}

func TestCheckForNat(t *testing.T) {
	plexus.SetLogging("debug")
	user, err := user.Current()
	assert.Nil(t, err)
	if user.Uid != "0" {
		t.Log("this test must be run as root")
		t.FailNow()
	}
	_, public, err := generateKeys()
	assert.Nil(t, err)
	self := Device{}
	self.WGPublicKey = public.String()
	peer := plexus.NetworkPeer{
		WGPublicKey: public.String(),
		HostName:    "peer1",
	}
	network := Network{}
	network.Name = "plexus"
	network.Net = net.IPNet{
		IP:   net.ParseIP("10.10.10.0").To4(),
		Mask: net.CIDRMask(20, 32),
	}
	network.Peers = append(network.Peers, peer)
	c := &nftables.Conn{}
	tables, err := c.ListTables()
	assert.Nil(t, err)
	for _, table := range tables {
		if table.Name == "plexus" {
			c.DelTable(table)
			err = c.Flush()
			assert.Nil(t, err)
		}
	}
	t.Run("noSubnetRouter", func(t *testing.T) {
		err := checkForNat(self, network)
		assert.Nil(t, err)
		tables, err := c.ListTables()
		assert.Nil(t, err)
		for _, table := range tables {
			if table.Name == "plexus" {
				t.FailNow()
			}
		}
	})
	t.Run("subnetWithoutNat", func(t *testing.T) {
		peer.IsSubnetRouter = true
		peer.Subnet = net.IPNet{
			IP:   net.ParseIP("192.168.0.0"),
			Mask: net.CIDRMask(24, 32),
		}
		network.Peers = []plexus.NetworkPeer{peer}
		err = checkForNat(self, network)
		assert.Nil(t, err)
		tables, err := c.ListTables()
		assert.Nil(t, err)
		for _, table := range tables {
			if table.Name == "plexus" {
				t.FailNow()
			}
		}
	})
	t.Run("subnetWithNat", func(t *testing.T) {
		table := &nftables.Table{}
		chain := &nftables.Chain{}
		peer.UseNat = true
		network.Peers = []plexus.NetworkPeer{peer}
		err = checkForNat(self, network)
		assert.Nil(t, err)
		tables, err := c.ListTables()
		assert.Nil(t, err)
		tableFound := false
		for _, t := range tables {
			if t.Name == "plexus" {
				tableFound = true
				table = t
			}
		}
		assert.True(t, tableFound)
		chains, err := c.ListChains()
		assert.Nil(t, err)
		chainFound := false
		for _, c := range chains {
			if c.Name == "plexus-nat" {
				chainFound = true
				chain = c
			}
		}
		assert.True(t, chainFound)
		rules, err := c.GetRules(table, chain)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(rules))
		assert.Equal(t, &expr.Masq{
			Random:      false,
			FullyRandom: false,
			Persistent:  false,
			ToPorts:     false,
			RegProtoMin: 0,
			RegProtoMax: 0,
		}, rules[0].Exprs[0])
	})
	t.Run("virtual subnet", func(t *testing.T) {
		table := &nftables.Table{}
		chain := &nftables.Chain{}
		peer.UseNat = false
		peer.UseVirtSubnet = true
		peer.VirtSubnet = net.IPNet{
			IP:   net.ParseIP("10.100.0.0").To4(),
			Mask: net.CIDRMask(24, 32),
		}
		network.Peers = []plexus.NetworkPeer{peer}
		t.Log(self, network)
		err = checkForNat(self, network)
		assert.Nil(t, err)
		tables, err := c.ListTables()
		assert.Nil(t, err)
		tableFound := false
		for _, t := range tables {
			if t.Name == "plexus" {
				tableFound = true
				table = t
			}
		}
		assert.True(t, tableFound)
		chains, err := c.ListChains()
		assert.Nil(t, err)
		chainFound := false
		for _, c := range chains {
			if c.Name == "plexus-subnet" {
				chainFound = true
				chain = c
			}
		}
		assert.True(t, chainFound)
		rules, err := c.GetRules(table, chain)
		assert.Nil(t, err)
		assert.Equal(t, 254, len(rules))
	})
	cleanNat(t, c)

}

func cleanNat(t *testing.T, c *nftables.Conn) {
	tables, err := c.ListTables()
	assert.Nil(t, err)
	for _, table := range tables {
		if table.Name == "plexus" {
			c.DelTable(table)
			err := c.Flush()
			assert.Nil(t, err)
			break
		}
	}
}
