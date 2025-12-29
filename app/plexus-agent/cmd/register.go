/*
Copyright © 2023 Matthew R Kasun <mkasun@nusak.ca>

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

// registerCmd represents the register command.
var registerCmd = &cobra.Command{
	Use:   "register token",
	Args:  cobra.ExactArgs(1),
	Short: "register with a plexus server",
	Long:  `register with a plexus server using token`,
	Run: func(cmd *cobra.Command, args []string) {
		request := plexus.RegisterRequest{
			Token: args[0],
		}
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		resp := plexus.MessageResponse{}
		cobra.CheckErr(
			agent.Request(ec, agent.Agent+plexus.Register, request, &resp, agent.NatsTimeout),
		)
		fmt.Println(resp.Message)
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// registerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
}
