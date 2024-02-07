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

	"github.com/spf13/cobra"
)

const version = "v0.1.0"

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "display version",
	Long:  `display version`,
	Run: func(cmd *cobra.Command, args []string) {
		long, err := cmd.Flags().GetBool("long")
		cobra.CheckErr(err)
		fmt.Print(version)
		if long {
			fmt.Print(": ")
			info, _ := debug.ReadBuildInfo()
			for _, setting := range info.Settings {
				if strings.Contains(setting.Key, "vcs") {
					fmt.Print(setting.Value + " ")
				}
			}
			fmt.Print("\n")
		}
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
	versionCmd.Flags().BoolP("long", "l", false, "display additional details")
}
