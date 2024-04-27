package plexus

import (
	"fmt"
	"log/slog"

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
func (w *wireguard) Attrs() *netlink.LinkAttrs {
	attr := netlink.NewLinkAttrs()
	attr.Name = w.Name
	attr.MTU = w.MTU
	if link, err := netlink.LinkByName(w.Name); err == nil {
		attr.Index = link.Attrs().Index
	}
	return &attr
}

// Type satisfies netlink Link interface
func (w *wireguard) Type() string {
	return "wireguard"
}

// Apply apply configuration to wireguard device
func (w *wireguard) Apply() error {
	slog.Debug("applying wg config", "wireguard", w)
	wgClient, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("wgtcl.New %w", err)
	}
	defer wgClient.Close()
	if err := wgClient.ConfigureDevice(w.Name, w.Config); err != nil {
		return fmt.Errorf("wireguard configure device, %w", err)
	}
	link, err := netlink.LinkByName(w.Name)
	if err != nil {
		return fmt.Errorf("get link %v", err)
	}
	newRoute := netlink.Route{
		LinkIndex: link.Attrs().Index,
		Scope:     netlink.SCOPE_LINK,
		Src:       w.Address.IP,
		Protocol:  2,
	}
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("get routes %v", err)
	}
	for _, route := range routes {
		slog.Debug("checking route", "route", route.Dst, "network address", w.Address.IPNet)
		if route.Dst.Contains(w.Address.IP) {
			// don't delete default route for the plexus network
			slog.Debug("skipping")
			continue
		}
		slog.Info("deleting route to ", "destination", route.Dst)

		if err := netlink.RouteDel(&route); err != nil {
			slog.Error("delete route", "destination", route.Dst, "error", err)
		}
	}
	for _, peer := range w.Config.Peers {
		for _, allowed := range peer.AllowedIPs {
			if w.Address.Contains(allowed.IP) {
				continue
			}
			newRoute.Dst = &allowed
			slog.Info("adding route", "route", newRoute)
			if err := netlink.RouteAdd(&newRoute); err != nil {
				slog.Error("add route", "destination", newRoute.Dst, "error", err)
			}
		}
	}
	return nil
}

// Up brings a wireguard interface up
func (w *wireguard) Up() error {
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
func (w *wireguard) Down() error {
	link, err := netlink.LinkByName(w.Name)
	if err != nil {
		return err
	}
	return netlink.LinkDel(link)
}

// New returns a new wireguard interface
func New(name string, mtu int, address netlink.Addr, config wgtypes.Config) *wireguard {
	wg := &wireguard{
		Name:    name,
		MTU:     mtu,
		Address: address,
		Config:  config,
	}
	slog.Debug("new wireguard interface", "wg", wg)
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
func Get(name string) (*wireguard, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}
	device, err := GetDevice(name)
	if err != nil {
		return nil, err
	}
	addrs, err := netlink.AddrList(link, nl.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	wg := &wireguard{
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

func (wg *wireguard) ReplacePeer(newPeer wgtypes.PeerConfig) {
	slog.Debug("replacing wg peer", "key", newPeer.PublicKey, "allowed", newPeer.AllowedIPs, "endpoint", newPeer.Endpoint)
	for i, peer := range wg.Config.Peers {
		if peer.PublicKey != newPeer.PublicKey {
			continue
		}
		wg.Config.Peers[i] = newPeer
		wg.Config.ReplacePeers = true
		break
	}
}

func (wg *wireguard) DeletePeer(key string) {
	for i, peer := range wg.Config.Peers {
		if peer.PublicKey.String() == key {
			wg.Config.Peers[i].Remove = true
			break
		}
	}
}

func (wg *wireguard) AddPeer(newPeer wgtypes.PeerConfig) {
	slog.Debug("adding new wg peer", "peer", newPeer)
	wg.Config.Peers = append(wg.Config.Peers, newPeer)
}
