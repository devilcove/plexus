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

var server string

// reloadCmd represents the reload command
var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "reload network configuration(s)",
	Long: `reload network configurations(s)
from single or all servers`,
	Run: func(cmd *cobra.Command, args []string) {
		if server != "" {
			fmt.Println("reloading data from server", server)
		} else {
			fmt.Println("reloading data from all connected servers")
		}
		request := plexus.ReloadRequest{
			Server: server,
		}
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		resp := plexus.ServerResponse{}
		cobra.CheckErr(ec.Request("reload", request, &resp, agent.NatsTimeout))
		if resp.Error {
			fmt.Println("errors were encountered")
		}
		fmt.Println(resp.Message)

	},
}

func init() {
	rootCmd.AddCommand(reloadCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// reloadCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	reloadCmd.Flags().StringVarP(&server, "server", "s", "", "server to query")

}
