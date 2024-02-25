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

// joinCmd represents the connect command
var joinCmd = &cobra.Command{
	Use:   "join network server",
	Args:  cobra.ExactArgs(2),
	Short: "join network",
	Long:  `join network`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("join called")
		var response plexus.NetworkResponse
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		defer ec.Close()
		cobra.CheckErr(ec.Request("update", plexus.UpdateRequest{
			Network: args[0],
			Server:  args[1],
			Action:  plexus.JoinNetwork,
		}, &response, agent.NatsTimeout))
		if response.Error {
			fmt.Println("errors were encountered")
		}
		fmt.Println(response.Message)
	},
}

func init() {
	rootCmd.AddCommand(joinCmd)
}
