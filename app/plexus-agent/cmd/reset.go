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

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/agent"
	"github.com/spf13/cobra"
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset network",
	Args:  cobra.ExactArgs(1),
	Short: "reset interface peers for specified network",
	Long:  `resets wg interface peers for given network`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("reset interface", args[0])
		request := plexus.ResetRequest{
			Network: args[0],
		}
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		resp := plexus.MessageResponse{}
		cobra.CheckErr(ec.Request(agent.Agent+plexus.Reset, request, &resp, agent.NatsTimeout))
		fmt.Println(resp.Message)
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// resetCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// resetCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
