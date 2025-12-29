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
	"runtime/debug"
	"strings"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/agent"
	"github.com/kr/pretty"
	"github.com/spf13/cobra"
)

const version = "v0.3.0"

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "display version information",
	Long: `display version information
	and optionally server(s) and agent version`,
	Run: func(cmd *cobra.Command, args []string) {
		if long {
			nc, err := agent.ConnectToAgentBroker()
			cobra.CheckErr(err)
			response := plexus.VersionResponse{}
			// need longer timeout is case of server timeout
			err = agent.Request(
				nc,
				agent.Agent+plexus.Version,
				long,
				&response,
				agent.NatsLongTimeout,
			)
			if err != nil {
				fmt.Println("error", err)
			}
			fmt.Printf("Server: %s\n", response.Server)
			fmt.Printf("Agent:  %s\n", response.Agent)
			fmt.Printf("Binary: ")
		}
		fmt.Printf("%s: ", version)
		info, _ := debug.ReadBuildInfo()
		for _, setting := range info.Settings {
			if strings.Contains(setting.Key, "vcs") {
				fmt.Printf("%s ", setting.Value)
			}
		}
		fmt.Print("\n")
		pretty.Println(info.Main.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// versionCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	versionCmd.Flags().
		BoolVarP(&long, "long", "l", false, "display server(s)/agent version information")
}
