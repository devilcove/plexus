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
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/internal/agent"
	"github.com/spf13/cobra"
)

// var config agent.Configuration

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "plexus-agent",
	Short: "plexus agent",
	Long: `plexus agent to setup and manage plexus wireguard
networks.  Communicates with plexus server for network updates.
CLI to join/leave networks.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	agent.Config.Verbosity = *rootCmd.PersistentFlags().StringP("verbosity", "v", "INFO", "logging verbosity")
	agent.Config.NatsPort = *rootCmd.PersistentFlags().IntP("natsport", "p", 4223, "nats port for cli <-> agent comms")
	plexus.SetLogging(agent.Config.Verbosity)
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	plexus.SetLogging("INFO")
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	agent.Config.DataDir = home + "/.local/share/" + filepath.Base(os.Args[0]) + "/"

	// set defaults
	slog.Debug("using configuration", "config", agent.Config)
	debug.SetTraceback("single")
}
