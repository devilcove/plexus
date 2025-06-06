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
	"github.com/devilcove/plexus/internal/agent"
	"github.com/spf13/cobra"
)

// leaveCmd represents the leave command
var leaveCmd = &cobra.Command{
	Use:   "leave network",
	Args:  cobra.ExactArgs(1),
	Short: "leave network",
	Long:  "leave network",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("leaving network", args[0])
		var response plexus.MessageResponse
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		cobra.CheckErr(agent.Request(ec, agent.Agent+plexus.LeaveNetwork, plexus.LeaveRequest{
			Network: args[0],
		}, &response, agent.NatsTimeout))
		fmt.Println(response.Message)
		ec.Close()
	},
}

func init() {
	rootCmd.AddCommand(leaveCmd)
}
