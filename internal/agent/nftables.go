package agent

import (
	"log/slog"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

func addNat() error {
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
			slog.Debug("Nat check", "subnet-router", peer.IsSubNetRouter)
			if !peer.IsSubNetRouter {
				slog.Debug("nat check -- not subnetrouter")
				return nil
			}
			slog.Debug("Nat check --- subnet", "useNat", peer.UseNat)
			if peer.UseNat {
				slog.Debug("adding NAT", "network", network.Name)
				return addNat()
			}
		}
	}
	return nil
}
