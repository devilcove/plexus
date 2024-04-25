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
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/agent"
	"github.com/fatih/color"
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
		status := agent.StatusResponse{}
		//networks := []Network{}
		cobra.CheckErr(ec.Request(agent.Agent+plexus.Status, nil, &status, agent.NatsTimeout))
		if status.Server == "" {
			fmt.Println("agent running... not connected to servers")
			return
		}
		color.Green("Server")
		var colour func(a ...interface{}) string
		if status.Connected {
			colour = color.New(color.FgGreen).SprintFunc()
		} else {
			colour = color.New(color.FgRed).SprintFunc()

		}
		fmt.Println("\t", status.Server, ":", colour(status.Connected))

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
			color.Magenta("interface %s", network.Interface)
			fmt.Println("\t network name:", network.Name)
			fmt.Println("\t public key:", wg.PrivateKey.PublicKey())
			fmt.Println("\t listen port:", wg.ListenPort)
			fmt.Println("\t public listen port:", network.PublicListenPort)
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
				color.Yellow("peer: %s %s %s", peer.WGPublicKey, peer.HostName, peer.Address.IP)
				if peer.IsRelay {
					fmt.Println("\trelay: true")
					showRelayedPeers(peer.RelayedPeers, network)
				}
				fmt.Println("\tprivate-endpoint:", peer.PrivateEndpoint.String()+":", peer.ListenPort)
				fmt.Println("\tpublic-endpoint:", peer.Endpoint.String()+":", peer.PublicListenPort)
				fmt.Println("\twg-endpoint:", wgPeer.Endpoint)
				fmt.Print("\tallowed ips:")
				for _, ip := range wgPeer.AllowedIPs {
					ones, _ := ip.Mask.Size()
					fmt.Print(" " + ip.IP.String() + "/" + strconv.Itoa(ones))
				}
				fmt.Println()
				printHandshake(wgPeer.LastHandshakeTime)
				fmt.Println("\ttransfer:", prettyByteSize(wgPeer.TransmitBytes), "sent", prettyByteSize(wgPeer.ReceiveBytes), "received")
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

func showRelayedPeers(relayed []string, network agent.Network) {
	for _, peer := range network.Peers {
		if slices.Contains(relayed, peer.WGPublicKey) {
			fmt.Printf("\t\t relayed: %s %s %v\n", peer.WGPublicKey, peer.HostName, peer.Address.IP)
		}
	}
}

func printHandshake(handshake time.Time) {
	d := time.Since(handshake)
	hour := int(d.Hours())
	minute := int(d.Minutes()) % 60
	second := int(d.Seconds()) % 60
	var hourString, minuteString, secondString string
	if hour == 0 {
		hourString = ""
	} else if hour == 1 {
		hourString = fmt.Sprintf("1 %s", color.GreenString("hour"))
	} else {
		hourString = fmt.Sprintf("%d %s", hour, color.GreenString("hours"))
	}
	if minute == 0 && hour == 0 {
		minuteString = ""
	} else if minute == 1 {
		minuteString = fmt.Sprintf("1 %s", color.GreenString("minute"))
	} else {
		minuteString = fmt.Sprintf("%d %s", minute, color.GreenString("minutes"))
	}
	if minute == 0 && hour == 0 && second == 0 {
		secondString = color.RedString("never")
	} else if second == 1 {
		secondString = fmt.Sprintf("1 %s", color.GreenString("second"))
	} else {
		secondString = fmt.Sprintf("%d %s", second, color.GreenString("seconds"))
	}
	fmt.Println("\tlast handshake:", hourString, minuteString, secondString, "ago")
}

func prettyByteSize(b int64) string {
	bf := float64(b)
	for i, unit := range []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei"} {
		if math.Abs(bf) < 1024.0 {
			units := color.GreenString("%sB", unit)
			if i == 0 {
				return fmt.Sprintf("%1.0f %s", bf, units)
			}
			return fmt.Sprintf("%3.2f %s", bf, units)
		}
		bf /= 1024.0
	}
	return ""
}
