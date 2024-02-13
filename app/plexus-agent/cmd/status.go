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
	"strconv"
	"time"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/agent"
	"github.com/spf13/cobra"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "display status",
	Long:  `display status`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("status called")
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		networks := []plexus.Network{}
		cobra.CheckErr(ec.Request("status", nil, &networks, time.Second*5))
		if len(networks) == 0 {
			fmt.Println("no networks")
			return
		}
		for _, network := range networks {
			wg, err := plexus.GetDevice(network.Interface)
			if err != nil {
				slog.Error("get wg device", "interface", network.Interface, "error", err)
				continue
			}
			fmt.Println("interface:", network.Interface)
			fmt.Println("\tnetwork name:", network.Name)
			fmt.Println("\tserver: ", network.ServerURL)
			fmt.Println("\tpublic key:", wg.PrivateKey.PublicKey())
			fmt.Println("\tlisten port:", wg.ListenPort)
			fmt.Println()
			for i, peer := range network.Peers {
				if peer.WGPublicKey == wg.PrivateKey.PublicKey().String() {
					continue
				}
				fmt.Println("peer:", peer.WGPublicKey, peer.HostName, peer.Address.IP)
				fmt.Println("\tendpoint:", peer.Endpoint+":", peer.PublicListenPort)
				fmt.Print("\tallowed ips:")
				for _, ip := range wg.Peers[i].AllowedIPs {
					ones, _ := ip.Mask.Size()
					fmt.Print(", " + ip.IP.String() + "/" + strconv.Itoa(ones))
				}
				fmt.Println()
				fmt.Println("\tlast handshake:", time.Since(wg.Peers[i].LastHandshakeTime).Seconds(), "seconds ago")
				fmt.Println("\ttransfer:", wg.Peers[i].ReceiveBytes, "received", wg.Peers[i].TransmitBytes, "sent")
				fmt.Println("\tkeepalive:", wg.Peers[i].PersistentKeepaliveInterval)
			}
			fmt.Println()
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
	// statusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
