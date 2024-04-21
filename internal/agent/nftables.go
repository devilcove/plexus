package agent

import (
	"log/slog"
	"net"

	"github.com/c-robinson/iplib"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func addNat() error {
	slog.Debug("adding NAT rule")
	c := &nftables.Conn{}
	table := c.AddTable(&nftables.Table{
		Name:   "plexus",
		Family: nftables.TableFamilyIPv4,
	})
	if err := delNat(); err != nil {
		return err
	}
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
	return c.Flush()
}

func delNat() error {
	slog.Debug("deleting NAT rules if required")
	c := &nftables.Conn{}
	chains, err := c.ListChains()
	if err != nil {
		return err
	}
	for _, chain := range chains {
		if chain.Name == "plexus-nat" {
			slog.Debug("deleting plexus-nat chain")
			c.DelChain(chain)
			err := c.Flush()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func checkForNat(self Device, network Network) error {
	slog.Debug("checking if NAT required")
	for _, peer := range network.Peers {
		if peer.WGPublicKey == self.WGPublicKey {
			slog.Debug("Nat check", "subnet-router", peer.IsSubnetRouter)
			if !peer.IsSubnetRouter {
				slog.Debug("nat check -- not subnetrouter")
				return nil
			}
			slog.Debug("Nat check --- subnet", "useNat", peer.UseNat)
			if peer.UseNat {
				slog.Debug("adding NAT", "network", network.Name)
				return addNat()
			}
			if peer.UseVirtSubnet {
				slog.Debug("adding virtual subnet", "peer", peer.HostName, "virtual subnet", peer.VirtSubnet, "subnet", peer.Subnet)
				return addVirtualSubnet(peer.VirtSubnet, peer.Subnet)
			}
		}
	}
	return nil
}

func addVirtualSubnet(virtual, subnet net.IPNet) error {
	slog.Debug("add virtual subnet", "virtual", virtual, "subnet", subnet)
	c := &nftables.Conn{}
	table := c.AddTable(&nftables.Table{
		Name:   "plexus",
		Family: nftables.TableFamilyIPv4,
	})
	if err := delVirtualSubnet(); err != nil {
		slog.Debug("delete virtual subnet", "error", err)
		return err
	}
	chain := c.AddChain(&nftables.Chain{
		Name:     "plexus-subnet",
		Table:    table,
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityFilter,
	})
	ones, _ := virtual.Mask.Size()
	virtNet := iplib.NewNet4(virtual.IP, ones)
	virt := virtNet.FirstAddress()
	subNet := iplib.NewNet4(subnet.IP, ones)
	sub := subNet.FirstAddress()
	rule := &nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Payload{
				OperationType:  expr.PayloadLoad,
				SourceRegister: 0,
				DestRegister:   1,
				Base:           expr.PayloadBaseNetworkHeader,
				Offset:         0x10,
				Len:            0x4,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     virt,
			},
			&expr.Immediate{
				Register: 1,
				Data:     sub,
			},
			&expr.NAT{
				Type:        expr.NATTypeDestNAT,
				Family:      uint32(nftables.TableFamilyIPv4),
				RegAddrMin:  1,
				RegAddrMax:  1,
				RegProtoMin: 0,
				RegProtoMax: 0,
			},
		},
	}
	c.AddRule(rule)
	if err := c.Flush(); err != nil {
		slog.Debug("flush rules", "errror", err)
		return err
	}
	for {
		var err error
		virt, err = virtNet.NextIP(virt)
		if err != nil {
			break
		}
		sub, err = subNet.NextIP(sub)
		if err != nil {
			break
		}
		rule.Exprs[1].(*expr.Cmp).Data = virt
		rule.Exprs[2].(*expr.Immediate).Data = sub
		c.AddRule(rule)
		if err := c.Flush(); err != nil {
			slog.Debug("flush rules", "errror", err)
			return err
		}
	}
	return nil
}

func delVirtualSubnet() error {
	slog.Debug("deleting virtual subnet")
	c := &nftables.Conn{}
	chains, err := c.ListChains()
	if err != nil {
		return nil
	}
	for _, chain := range chains {
		if chain.Name == "plexus-subnet" {
			slog.Debug("deleting plexus-subnet chain")
			c.DelChain(chain)
			return c.Flush()
		}
	}
	return nil
}
