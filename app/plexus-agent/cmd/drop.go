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

// dropCmd represents the drop command
var dropCmd = &cobra.Command{
	Use:   "drop server",
	Args:  cobra.ExactArgs(1),
	Short: "unregister from server",
	Long:  `unregister from server. Also deletes networks controlled by server`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("leaving server", args[0])
		var response plexus.ServerResponse
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		cobra.CheckErr(ec.Request("update", plexus.AgentRequest{
			Action: plexus.LeaveServer,
			Server: args[0],
		},
			&response, agent.NatsTimeout))
		if response.Error {
			fmt.Println("errors were encountered")
		}
		fmt.Println(response.Message)
		ec.Close()
	},
}

func init() {
	rootCmd.AddCommand(dropCmd)
}
