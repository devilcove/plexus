package plexus

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// wireguard is a netlink compatible representation of wireguard interface
type wireguard struct {
	Name    string
	MTU     int
	Address netlink.Addr
	Config  wgtypes.Config
}

// Attrs satisfies netlink Link interface
func (w wireguard) Attrs() *netlink.LinkAttrs {
	attr := netlink.NewLinkAttrs()
	attr.Name = w.Name
	attr.MTU = w.MTU
	if link, err := netlink.LinkByName(w.Name); err == nil {
		attr.Index = link.Attrs().Index
	}
	return &attr
}

// Type satisfies netlink Link interface
func (w wireguard) Type() string {
	return "wireguard"
}

// Apply apply configuration to wireguard device
func (w wireguard) Apply() error {
	wg, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wg.Close()
	return wg.ConfigureDevice(w.Name, w.Config)
}

// Up brings a wireguard interface up
func (w wireguard) Up() error {
	if err := netlink.LinkAdd(w); err != nil {
		return fmt.Errorf("link add %v", err)
	}
	if err := netlink.AddrAdd(w, &w.Address); err != nil {
		return fmt.Errorf("add address %v", err)
	}
	if err := netlink.LinkSetUp(w); err != nil {
		return fmt.Errorf("link up %v", err)
	}
	return w.Apply()
}

// Down removes a wireguard interface
func (w wireguard) Down() error {
	link, err := netlink.LinkByName(w.Name)
	if err != nil {
		return err
	}
	return netlink.LinkDel(link)
}

// New returns a new wireguard interface
func New(name string, mtu int, address netlink.Addr, config wgtypes.Config) wireguard {
	wg := wireguard{
		Name:    name,
		MTU:     mtu,
		Address: address,
		Config:  config,
	}
	return wg
}

// GetDevice returns a wireguard device as wgtype.Device
func GetDevice(name string) (*wgtypes.Device, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, err
	}
	return client.Device(name)
}

// Get returns an existing wireguard interface as a plexus.Wireguard
func Get(name string) (wireguard, error) {
	empty := wireguard{}
	link, err := netlink.LinkByName(name)
	if err != nil {
		return empty, err
	}
	device, err := GetDevice(name)
	if err != nil {
		return empty, err
	}
	addrs, err := netlink.AddrList(link, nl.FAMILY_V4)
	if err != nil {
		return empty, err
	}
	wg := wireguard{
		Name:    name,
		MTU:     link.Attrs().MTU,
		Address: addrs[0],
		Config: wgtypes.Config{
			PrivateKey: &device.PrivateKey,
			ListenPort: &device.ListenPort,
			Peers:      convertPeers(device.Peers),
		},
	}
	return wg, nil
}

func convertPeers(input []wgtypes.Peer) []wgtypes.PeerConfig {
	output := []wgtypes.PeerConfig{}
	for _, peer := range input {
		newpeer := wgtypes.PeerConfig{
			PublicKey:                   peer.PublicKey,
			Endpoint:                    peer.Endpoint,
			PersistentKeepaliveInterval: &peer.PersistentKeepaliveInterval,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  peer.AllowedIPs,
		}
		output = append(output, newpeer)
	}
	return output
}
