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

	"github.com/devilcove/plexus/agent"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "plexus-agent deamon",
	Long: `plexus-agent run maintains a connection to 
plexus server(s) for network updates.`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run called")
		agent.Run()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
