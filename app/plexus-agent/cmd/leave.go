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

// leaveCmd represents the leave command
var leaveCmd = &cobra.Command{
	Use:   "leave",
	Args:  cobra.ExactArgs(1),
	Short: "leave network",
	Long:  "leave network",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("leave called")
		var response plexus.LeaveResponse
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		cobra.CheckErr(ec.Request("update", plexus.UpdateRequest{
			Network: args[0],
			Action:  plexus.LeaveNetwork,
		}, &response, agent.NatsTimeout))
		if response.Error {
			fmt.Println("errors were encounterd")
		}
		fmt.Println(response.Message)
		//cobra.CheckErr(ec.Flush())
		ec.Close()
	},
}

func init() {
	rootCmd.AddCommand(leaveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// leaveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	leaveCmd.Flags().StringP("network", "n", "", "name of network to leave")
}
