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
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "plexus-agent deamon",
	Long: `plexus-agent run maintains a connection to 
a plexus server for network updates.`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run called")
		run()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func run() {
	if err := plexus.WritePID(os.Getenv("HOME")+"/.cache/plexus-agent.pid", os.Getpid()); err != nil {
		slog.Error("failed to write pid to file", "error", err)
	}
	wg := sync.WaitGroup{}
	quit := make(chan os.Signal, 1)
	reset := make(chan os.Signal, 1)
	pause := make(chan os.Signal, 1)
	unpause := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	signal.Notify(reset, syscall.SIGHUP)
	signal.Notify(pause, syscall.SIGUSR1)
	signal.Notify(unpause, syscall.SIGUSR2)
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go natSubscribe(ctx, &wg)
	for {
		select {
		case <-quit:
			log.Println("quit")
			cancel()
			wg.Wait()
			time.Sleep(time.Second)
			log.Println("go routines stopped")
			return
		case <-reset:
			log.Println("reset")
			cancel()
			wg.Wait()
			log.Println("go routines stopped by reset")
			wg.Add(1)
			ctx, cancel = context.WithCancel(context.Background())
			go natSubscribe(ctx, &wg)
		case <-pause:
			log.Println("pause")
			cancel()
			wg.Wait()
			log.Println("go routines stopped by pause")
		case <-unpause:
			log.Println("unpause")
			//cancel in case pause not received earlier
			cancel()
			wg.Wait()
			wg.Add(1)
			ctx, cancel = context.WithCancel(context.Background())
			go natSubscribe(ctx, &wg)
		}
	}
}

func natSubscribe(ctx context.Context, wg *sync.WaitGroup) {
	if err := boltdb.Initialize(os.Getenv("HOME")+"/.local/share/plexus/plexus-agent.db", []string{"devices", "networks"}); err != nil {
		slog.Error("failed to initialize database")
		return
	}
	subscriptions := []*nats.Subscription{}
	servers := []*nats.Conn{}
	log.Println("mq starting")
	log.Println("config", config)
	defer wg.Done()
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("unable to read devices", "errror", err)
	}
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("unable to read networks", "error", err)
	}
	for i, network := range networks {
		if self.WGPublicKey == "" {
			continue
		}
		nc, err := connectToServer(self, network.ServerURL)
		if err != nil {
			slog.Error("connect to server", "error", err)
			continue
		}
		sub, err := nc.Subscribe("networks."+network.Name, networkUpdates)
		if err != nil {
			slog.Error("network subcription failed", "error", err)
			continue
		}
		if err := startInterface("plexus"+strconv.Itoa(i), self, network); err != nil {
			slog.Error("interface did not start", "name", "pleuxus.Name"+strconv.Itoa(i), "network", network.Name, "error", err)
			sub.Drain()
			continue
		}
		wg.Add(1)
		go checkin(ctx, wg, nc, self)
		subscriptions = append(subscriptions, sub)
		servers = append(servers, nc)
	}
	<-ctx.Done()
	log.Println("mq shutting down")
	for _, sub := range subscriptions {
		sub.Drain()
	}
	for _, nc := range servers {
		nc.Close()
	}
	boltdb.Close()
}

func checkin(ctx context.Context, wg *sync.WaitGroup, nc *nats.Conn, self plexus.Device) {
	defer wg.Done()
	log.Println("checking starting")
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("checkin done")
			return
		case <-ticker.C:
			msg, err := nc.Request("checkin."+self.WGPublicKey, []byte("checking"), time.Second)
			if err != nil {
				log.Println("error publishing checkin ", err)
				continue
			}
			log.Println("checkin response", string(msg.Data))
		}
	}
}

func connectToServer(self plexus.Device, server string) (*nats.Conn, error) {
	kp, err := nkeys.FromSeed([]byte(self.Seed))
	if err != nil {
		return nil, err
	}
	pk, err := kp.PublicKey()
	if err != nil {
		return nil, err
	}
	sign := func(nonce []byte) ([]byte, error) {
		return kp.Sign(nonce)
	}
	opts := nats.Options{
		Url:         "nats://" + server + ":4222",
		Nkey:        pk,
		SignatureCB: sign,
	}
	return opts.Connect()
}
