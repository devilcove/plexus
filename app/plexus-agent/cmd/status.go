/*
Copyright © 2024 Matthew R Kasun <mkasun@nusak.ca>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/agent"
	"github.com/kr/pretty"
	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var long bool

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "display status",
	Long:  `display status`,
	Run: func(cmd *cobra.Command, args []string) {
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		status := plexus.StatusResponse{}
		//networks := []plexus.Network{}
		cobra.CheckErr(ec.Request("status", nil, &status, agent.NatsTimeout))
		if len(status.Servers) == 0 {
			fmt.Println("not connect to any servers")
			return
		}
		fmt.Println("Servers")
		for _, server := range status.Servers {
			fmt.Println("\t", server)
		}
		if len(status.Networks) == 0 {
			fmt.Println("no networks")
			return
		}
		fmt.Println()
		for _, network := range status.Networks {
			wg, err := plexus.GetDevice(network.Interface)
			if err != nil {
				slog.Error("get wg device", "interface", network.Interface, "error", err)
				continue
			}
			link, err := netlink.LinkByName(network.Interface)
			if err != nil {
				slog.Error("get interface", "interface", network.Interface, "error", err)
			}
			addr, err := netlink.AddrList(link, netlink.FAMILY_ALL)
			if err != nil {
				slog.Error("get address of interface", "interface", network.Interface, "error", err)
			}
			fmt.Println("interface:", network.Interface)
			fmt.Println("\t network name:", network.Name)
			fmt.Println("\t server: ", network.ServerURL)
			fmt.Println("\t public key:", wg.PrivateKey.PublicKey())
			fmt.Println("\t listen port:", wg.ListenPort)
			for i := range addr {
				fmt.Println("\t address:", addr[i].IP)
			}
			wgPeer := wgtypes.Peer{}
			for _, peer := range network.Peers {
				if peer.IsRelayed {
					continue
				}
				if peer.WGPublicKey == wg.PrivateKey.PublicKey().String() {
					//self, skip
					continue
				}
				for _, x := range wg.Peers {
					if x.PublicKey.String() == peer.WGPublicKey {
						wgPeer = x
						break
					}
				}
				fmt.Println("peer:", peer.WGPublicKey, peer.HostName, peer.Address.IP)
				if peer.IsRelay {
					fmt.Println("\trelay: true")
					showRelayedPeers(peer.RelayedPeers, network)
				}
				fmt.Println("\tendpoint:", peer.Endpoint+":", peer.PublicListenPort)
				fmt.Print("\tallowed ips:")
				for _, ip := range wgPeer.AllowedIPs {
					ones, _ := ip.Mask.Size()
					fmt.Print(" " + ip.IP.String() + "/" + strconv.Itoa(ones))
				}
				fmt.Println()
				if wgPeer.LastHandshakeTime.IsZero() {
					fmt.Println("\tlast handshake: never")
				} else {
					fmt.Printf("\tlast handshake: %f0.0 %s\n", time.Since(wgPeer.LastHandshakeTime).Seconds(), "seconds ago")
				}
				fmt.Println("\ttransfer:", wgPeer.TransmitBytes, "sent", wgPeer.ReceiveBytes, "received")
				fmt.Println("\tkeepalive:", wgPeer.PersistentKeepaliveInterval)
			}
			fmt.Println()
			if long {
				pretty.Println(network)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// statusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	statusCmd.Flags().BoolVarP(&long, "long", "l", false, "display additional network detail")
}

func showRelayedPeers(relayed []string, network plexus.Network) {
	for _, peer := range network.Peers {
		if slices.Contains(relayed, peer.WGPublicKey) {
			fmt.Printf("\t\t relayed: %s %s %v\n", peer.WGPublicKey, peer.HostName, peer.Address.IP)
		}
	}
}
