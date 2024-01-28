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
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/kr/pretty"
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
		checkErr(err)
		join(token)
	},
}

func join(token string) {
	fmt.Println("join called")
	// stop daemon if running
	home, err := os.UserHomeDir()
	checkErr(err)
	pid, err := plexus.ReadPID(home + "/.cache/plexus-agent.pid")
	checkErr(err)
	if plexus.IsAlive(pid) {
		unix.Kill(pid, syscall.SIGUSR1)
		defer unix.Kill(pid, syscall.SIGUSR2)
	}
	dbfile, ok := os.LookupEnv("DB_FILE")
	if !ok {
		dbfile = os.Getenv("HOME") + "/.local/share/plexus/plexus-agent.db"
	}
	err = boltdb.Initialize(dbfile, []string{"devices", "networks"})
	checkErr(err)
	device := newDevice()
	loginKey, err := plexus.DecodeToken(token)
	checkErr(err)
	loginKeyPair, err := nkeys.FromSeed([]byte(loginKey.Seed))
	checkErr(err)
	loginPublicKey, err := loginKeyPair.PublicKey()
	checkErr(err)
	sign := func(nonce []byte) ([]byte, error) {
		return loginKeyPair.Sign(nonce)
	}
	request := plexus.JoinRequest{
		KeyName: loginKey.KeyName,
		Peer:    device.Peer,
	}
	opts := nats.Options{
		Url:         loginKey.URL,
		Nkey:        loginPublicKey,
		SignatureCB: sign,
	}
	pretty.Println("join request", request)
	payload, err := json.Marshal(&request)
	checkErr(err)
	log.Println(opts.Nkey)
	nc, err := opts.Connect()
	checkErr(err)
	slog.Info("connected to broker")
	msg, err := nc.Request("join", payload, time.Second*5)
	if err != nil {
		slog.Error("join request", "error", err)
	}
	checkErr(err)
	fmt.Println("response", "reply:", string(msg.Reply), "data:", string(msg.Data))
	networks := []plexus.Network{}
	if err := json.Unmarshal(msg.Data, &networks); err != nil {
		fmt.Println("join unsuccessful", string(msg.Data))
	} else {
		for _, network := range networks {
			if err := boltdb.Save(network, network.Name, "networks"); err != nil {
				fmt.Println("error saving network", network.Name, err)
			}
		}
	}
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

func newDevice() plexus.Device {
	device, err := boltdb.Get[plexus.Device]("self", "devices")
	if err == nil {
		return device
	}
	if !errors.Is(err, boltdb.ErrNoResults) {
		checkErr(err)
	}
	peer, privKey, seed := createPeer()
	device = plexus.Device{
		Peer:         peer,
		Seed:         seed,
		WGPrivateKey: privKey.String(),
	}
	err = boltdb.Save(device, "self", "devices")
	checkErr(err)
	return device
}

func createPeer() (plexus.Peer, wgtypes.Key, string) {
	kp, err := nkeys.CreateUser()
	checkErr(err)
	seed, err := kp.Seed()
	checkErr(err)
	nkey, err := kp.PublicKey()
	checkErr(err)
	name, err := os.Hostname()
	checkErr(err)
	privKey, pubKey, err := generateKeys()
	//privKey, err := wgtypes.GeneratePrivateKey()
	checkErr(err)
	if strings.Contains(pubKey.String(), "/") {
		checkErr("invalid public key" + pubKey.String())
	}
	//pubKey := privKey.PublicKey()
	port := checkPort(51820)
	stunAddr := getPublicAddPort()
	peer := plexus.Peer{
		WGPublicKey:      pubKey.String(),
		PubNkey:          nkey,
		Name:             name,
		Version:          "v0.1.0",
		ListenPort:       port,
		PublicListenPort: stunAddr.Port,
		Endpoint:         stunAddr.IP.String(),
		OS:               runtime.GOOS,
		Updated:          time.Now(),
	}
	return peer, privKey, string(seed)

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
	checkErr(err)
	local := &net.UDPAddr{
		IP:   net.ParseIP(""),
		Port: 51820,
	}
	c, err := net.DialUDP("udp4", local, stunServer)
	checkErr(err)

	conn, err := stun.NewClient(c)
	checkErr(err)
	msg := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	err = conn.Do(msg, func(res stun.Event) {
		checkErr(res.Error)
		err := add.GetFrom(res.Message)
		checkErr(err)
	})
	checkErr(err)
	err = conn.Close()
	checkErr(err)
	return
}
