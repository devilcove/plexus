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

// loglevelCmd represents the loglevel command
var loglevelCmd = &cobra.Command{
	Use:   "loglevel level",
	Args:  cobra.ExactArgs(1),
	Short: "set log level of daemon",
	Long: `set log level of damemon
DEBUG, INFO, WARN, or ERROR (upper or lowercase)
.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("setting daemon log level to", args[0])
		ec, err := agent.ConnectToAgentBroker()
		cobra.CheckErr(err)
		cobra.CheckErr(ec.Publish("loglevel", plexus.LevelRequest{Level: args[0]}))
		cobra.CheckErr(ec.Flush())
		cobra.CheckErr(ec.Drain())
	},
}

func init() {
	rootCmd.AddCommand(loglevelCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// loglevelCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// loglevelCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
