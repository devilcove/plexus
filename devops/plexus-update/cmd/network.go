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
	"os"
	"strings"

	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/spf13/cobra"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// networkCmd represents the network command
var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	//Run: func(cmd *cobra.Command, args []string) {
	//	fmt.Println("network called")
	//},
}

func init() {
	rootCmd.AddCommand(networkCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// networkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// networkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func broker() *nats.Conn {
	seed, err := os.ReadFile("/tmp/seed")
	cobra.CheckErr(err)
	keyPair, err := nkeys.FromSeed(seed)
	cobra.CheckErr(err)
	pubKey, err := keyPair.PublicKey()
	cobra.CheckErr(err)
	sign := func(nonce []byte) ([]byte, error) {
		return keyPair.Sign(nonce)
	}
	opts := nats.Options{
		Nkey:        pubKey,
		SignatureCB: sign,
	}
	nc, err := opts.Connect()
	cobra.CheckErr(err)
	return nc
}

func newPeer() plexus.NetworkPeer {
	_, pub, err := generateKeys()
	cobra.CheckErr(err)
	addr, err := netlink.ParseIPNet("10.10.10.2/24")
	cobra.CheckErr(err)
	return plexus.NetworkPeer{
		WGPublicKey:      pub.String(),
		HostName:         "somehost",
		Address:          *addr,
		PublicListenPort: 51820,
		Endpoint:         "192.168.0.105:51820",
	}
}

// generateKeys generates wgkeys that do not have a / in pubkey
func generateKeys() (wgtypes.Key, wgtypes.Key, error) {
	for {
		priv, err := wgtypes.GenerateKey()
		if err != nil {
			return priv, wgtypes.Key{}, err
		}
		pub := priv.PublicKey()
		if !strings.Contains(pub.String(), "/") {
			return priv, pub, nil
		}
	}
}
