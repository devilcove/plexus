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
	"encoding/json"
	"fmt"

	"github.com/devilcove/plexus"
	"github.com/spf13/cobra"
)

// addPeerCmd represents the addPeer command
var addPeerCmd = &cobra.Command{
	Use:   "addPeer networkName",
	Short: "add peer to plexus agent",
	Args:  cobra.ExactArgs(1),
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("addPeer called")
		nc := broker()
		request := plexus.NetworkUpdate{
			Action: plexus.AddPeer,
			Peer:   newPeer(),
		}
		payload, err := json.Marshal(request)
		cobra.CheckErr(err)
		err = nc.Publish("networks."+args[0], payload)
		cobra.CheckErr(err)
		nc.Close()
	},
}

func init() {
	networkCmd.AddCommand(addPeerCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addPeerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// addPeerCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
