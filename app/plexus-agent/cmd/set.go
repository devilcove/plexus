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
	"net"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/agent"
	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
)

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set ip [network]",
	Args:  cobra.RangeArgs(1, 2),
	Short: "set private endpoint for network",
	Long:  `set private endpoint ip for a or all networks.`,
	Run: func(cmd *cobra.Command, args []string) {
		network := ""
		if len(args) > 1 {
			network = args[1]
		}
		fmt.Println("set called")
		ip := net.ParseIP(args[0])
		if ip == nil {
			fmt.Println("invalid ip")
			return
		}
		addr, err := netlink.AddrList(nil, netlink.FAMILY_V4)
		if err != nil {
			fmt.Println("error getting addresses", err)
			return
		}
		found := false
		for _, add := range addr {
			if ip.Equal(add.IP) {
				found = true
			}
		}
		if !found {
			fmt.Println("invalid ip")
			return
		}
		request := plexus.PrivateEndpoint{
			IP:      args[0],
			Network: network,
		}
		resp := plexus.MessageResponse{}
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		cobra.CheckErr(ec.Request(agent.Agent+plexus.SetPrivateEndpoint, request, &resp, agent.NatsTimeout))
		fmt.Println(resp.Message)
	},
}

func init() {
	rootCmd.AddCommand(setCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// setCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// setCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
