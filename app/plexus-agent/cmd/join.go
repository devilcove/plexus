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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/pion/stun"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// joinCmd represents the join command
var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "join a plexus server",
	Long:  `join a plexus server using token`,
	Run: func(cmd *cobra.Command, args []string) {
		token, err := cmd.Flags().GetString("token")
		cobra.CheckErr(err)
		join(token)
	},
}

func join(token string) {
	fmt.Println("join called")
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	pid, err := plexus.ReadPID(home + "/.cache/plexus-agent.pid")
	cobra.CheckErr(err)
	if plexus.IsAlive(pid) {
		unix.Kill(pid, syscall.SIGUSR1)
	}
	kv, err := plexus.DecodeToken(token)
	cobra.CheckErr(err)
	kp, err := nkeys.FromSeed([]byte(kv.Seed))
	cobra.CheckErr(err)
	pk, err := kp.PublicKey()
	cobra.CheckErr(err)
	sign := func(nonce []byte) ([]byte, error) {
		return kp.Sign(nonce)
	}
	peer, privKey := createPeer(kv.Seed)
	device := plexus.Device{
		Peer:       peer,
		Seed:       kv.Seed,
		PrivateKey: privKey,
		PrivKeyStr: privKey.String(),
	}
	saveDevice(device)
	request := plexus.JoinRequest{
		KeyName: kv.KeyName,
		Peer:    peer,
	}
	opts := nats.Options{
		Url:         kv.URL,
		Nkey:        pk,
		SignatureCB: sign,
	}
	payload, err := json.Marshal(&request)
	cobra.CheckErr(err)
	nc, err := opts.Connect()
	cobra.CheckErr(err)
	msg, err := nc.Request("join", payload, time.Second*5)
	cobra.CheckErr(err)
	fmt.Println(string(msg.Data))
	if plexus.IsAlive(pid) {
		unix.Kill(pid, syscall.SIGUSR2)
	}
}

func init() {
	rootCmd.AddCommand(joinCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// joinCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	joinCmd.Flags().StringP("token", "t", "", "token to join server")
}

func createPeer(seed string) (plexus.Peer, wgtypes.Key) {
	kp, err := nkeys.FromSeed([]byte(seed))
	cobra.CheckErr(err)
	nkey, err := kp.PublicKey()
	cobra.CheckErr(err)
	name, err := os.Hostname()
	cobra.CheckErr(err)
	privKey, err := wgtypes.GeneratePrivateKey()
	cobra.CheckErr(err)
	pubKey := privKey.PublicKey()
	port := checkPort(51820)
	stunAddr := getPublicAddPort()
	peer := plexus.Peer{
		PublicKey:        pubKey,
		PubKeyStr:        pubKey.String(),
		PubNkey:          nkey,
		Name:             name,
		Version:          "v0.1.0",
		ListenPort:       port,
		PublicListenPort: stunAddr.Port,
		Endpoint:         stunAddr.IP,
		OS:               runtime.GOOS,
		Updated:          time.Now(),
	}
	return peer, privKey

}

func checkPort(rangestart int) int {
	addr := net.UDPAddr{}
	for x := rangestart; x <= 65535; x++ {
		addr.Port = x
		conn, err := net.ListenUDP("udp", &addr)
		if err != nil {
			continue
		}
		conn.Close()
		return x
	}
	return 0
}

func getPublicAddPort() (add stun.XORMappedAddress) {
	stunServer, err := net.ResolveUDPAddr("udp4", "stun1.l.google.com:19302")
	cobra.CheckErr(err)
	local := &net.UDPAddr{
		IP:   net.ParseIP(""),
		Port: 51820,
	}
	c, err := net.DialUDP("udp4", local, stunServer)
	cobra.CheckErr(err)

	conn, err := stun.NewClient(c)
	cobra.CheckErr(err)
	msg := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	err = conn.Do(msg, func(res stun.Event) {
		cobra.CheckErr(res.Error)
		err := add.GetFrom(res.Message)
		cobra.CheckErr(err)
	})
	cobra.CheckErr(err)
	err = conn.Close()
	cobra.CheckErr(err)
	return
}

func saveDevice(device plexus.Device) error {
	home := os.Getenv("HOME")
	dbfile, ok := os.LookupEnv("DB_FILE")
	if !ok {
		dbfile = home + "/.local/share/plexus/plexus-agent.db"
	}
	err := boltdb.Initialize(dbfile, []string{"devices"})
	if err != nil {
		return err
	}
	defer boltdb.Close()
	return boltdb.Save(device, "self", "devices")
}