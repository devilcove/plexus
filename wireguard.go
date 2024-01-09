package plexus

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Wireguard is a netlink compatible representation of wireguard interface
type Wireguard struct {
	name    string
	mtu     int
	address netlink.Addr
	config  wgtypes.Config
}

// Attrs satisfies netlink Link interface
func (w Wireguard) Attrs() *netlink.LinkAttrs {
	attr := netlink.NewLinkAttrs()
	attr.Name = w.name
	attr.MTU = w.mtu
	if link, err := netlink.LinkByName(w.name); err == nil {
		attr.Index = link.Attrs().Index
	}
	return &attr
}

// Type satisfies netlink Link interface
func (w Wireguard) Type() string {
	return "wireguard"
}

// apply apply configuration to wireguard device
func (w Wireguard) apply() error {
	wg, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wg.Close()
	return wg.ConfigureDevice(w.name, w.config)
}

// Up brings a wireguard interface up
func (w Wireguard) Up() error {
	if err := netlink.LinkAdd(w); err != nil {
		return fmt.Errorf("link add %v", err)
	}
	if err := netlink.AddrAdd(w, &w.address); err != nil {
		return fmt.Errorf("add address %v", err)
	}
	if err := netlink.LinkSetUp(w); err != nil {
		return fmt.Errorf("link up %v", err)
	}
	return w.apply()
}

// Down removes a wireguard interface
func (w Wireguard) Down() error {
	link, err := netlink.LinkByName(w.name)
	if err != nil {
		return err
	}
	return netlink.LinkDel(link)
}

// New returns a new wireguard interface
func New(name string, mtu int, address netlink.Addr, config wgtypes.Config) Wireguard {
	wg := Wireguard{
		name:    name,
		mtu:     mtu,
		address: address,
		config:  config,
	}
	return wg
}
