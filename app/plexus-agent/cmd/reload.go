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

// reloadCmd represents the reload command
var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "reload network configuration(s)",
	Long:  `reload network configurations(s)`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("reloading data from server")
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		resp := plexus.MessageResponse{}
		cobra.CheckErr(ec.Request(agent.Agent+plexus.Reload, nil, &resp, agent.NatsTimeout))
		fmt.Println(resp.Message)
	},
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}
