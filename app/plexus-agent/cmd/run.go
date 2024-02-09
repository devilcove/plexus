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
	"encoding/json"
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

var (
	// networkMap containss the interface name and reset channel for networks
	networkMap map[string]plexus.NetMap
	serverMap  map[string]*nats.Conn
	restart    chan int
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "plexus-agent deamon",
	Long: `plexus-agent run maintains a connection to 
plexus server(s) for network updates.`,

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
	restart = make(chan int, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	signal.Notify(reset, syscall.SIGHUP)
	ctx, cancel := context.WithCancel(context.Background())
	if err := boltdb.Initialize(os.Getenv("HOME")+"/.local/share/plexus/plexus-agent.db", []string{"devices", "networks"}); err != nil {
		slog.Error("failed to initialize database", "error", err)
		return
	}
	self := newDevice()
	wg.Add(1)
	slog.Info("starting socket server")
	go socketServer(ctx, &wg)
	slog.Info("setup nats")
	setupNats(self)
	slog.Info("refresh data from servers")
	refreshData(self)
	slog.Info("set up subcriptions")
	setupSubs(ctx, &wg, self)
	for {
		select {
		case <-quit:
			slog.Info("quit")
			deleteAllInterface()
			cancel()
			wg.Wait()
			slog.Info("go routines stopped")
			return
		case <-reset:
			slog.Info("reset")
			cancel()
			wg.Wait()
			slog.Info("go routines stopped by reset")
			ctx, cancel = context.WithCancel(context.Background())
			wg.Add(1)
			go socketServer(ctx, &wg)
			setupNats(self)
			refreshData(self)
			closeNatsConns()
			setupNats(self)
			setupSubs(ctx, &wg, self)
		case <-restart:
			slog.Info("restart")
			cancel()
			wg.Wait()
			slog.Info("go routines stopped by restart")
			ctx, cancel = context.WithCancel(context.Background())
			wg.Add(1)
			go socketServer(ctx, &wg)
			setupNats(self)
			refreshData(self)
			setupSubs(ctx, &wg, self)
		}
	}
}

func closeNatsConns() {
	for _, nc := range serverMap {
		nc.Close()
	}
}

func setupNats(self plexus.Device) {
	serverMap = make(map[string]*nats.Conn)
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("unable to read networks", "error", err)
	}
	for _, network := range networks {
		nc, ok := serverMap[network.ServerURL]
		if !ok || nc == nil {
			nc, err := connectToServer(self, network.ServerURL)
			if err != nil {
				slog.Error("unable to connect to server", "server", network.ServerURL, "error", err)
			}
			serverMap[network.ServerURL] = nc
		}
	}
	fmt.Println("servermap", serverMap, "length", len(serverMap))
}

func refreshData(self plexus.Device) {
	slog.Info("getting data from servers")
	//delete existing networks
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Warn("get networks", "error", err)
	}
	for _, network := range networks {
		if err := boltdb.Delete[plexus.Network](network.Name, "networks"); err != nil {
			slog.Warn("delete network", "network", network.Name, "error", err)
		}
	}
	for key, nc := range serverMap {
		slog.Info("procssing server", "server", key)
		if nc == nil {
			slog.Error("nil nats connection", "key", key)
			continue
		}
		msg, err := nc.Request("config."+self.WGPublicKey, []byte("helloworld"), time.Second*5)
		if err != nil {
			slog.Error("refresh data", "server", key, "error", err)
			continue
		}
		slog.Info("refresh data", "msg", string(msg.Data))
		var networks []plexus.Network
		if err := json.Unmarshal(msg.Data, &networks); err != nil {
			slog.Error("unmarshal data from", "server", key, "error", err)
		}
		for i, network := range networks {
			network.Interface = "plexus" + strconv.Itoa(i)
			network.ListenPort = self.ListenPort
			if err := boltdb.Save(network, network.Name, "networks"); err != nil {
				slog.Error("save network", "network", network.Name, "error", err)
			}
		}
	}
}

func setupSubs(ctx context.Context, wg *sync.WaitGroup, self plexus.Device) {
	networkMap = make(map[string]plexus.NetMap)
	networks, err := boltdb.GetAll[plexus.Network]("networks")
	if err != nil {
		slog.Error("unable to read networks", "error", err)
	}
	for _, network := range networks {
		//start interface
		iface := network.Interface
		channel := make(chan bool, 1)
		networkMap[network.Name] = plexus.NetMap{
			Interface: iface,
			Channel:   channel,
		}
		if err := startInterface(self, network); err != nil {
			slog.Error("interface did not start", "name", iface, "network", network.Name, "error", err)
			return
		}
		wg.Add(3)
		go natSubscribe(ctx, wg, self, network, serverMap[network.ServerURL])
		go checkin(ctx, wg, serverMap[network.ServerURL], self, channel)
		go networkConnectivityStats(ctx, wg, self, network)
	}
}

func natSubscribe(ctx context.Context, wg *sync.WaitGroup, self plexus.Device, network plexus.Network, nc *nats.Conn) {
	log.Println("mq starting")
	log.Println("config", config)
	defer wg.Done()

	sub, err := nc.Subscribe("networks."+network.Name, networkUpdates)
	if err != nil {
		slog.Error("network subcription failed", "error", err)
		return
	}
	<-ctx.Done()
	log.Println("mq shutting down")
	if err := sub.Drain(); err != nil {
		slog.Error("drain subscriptions", "error", err)
	}
	nc.Close()
	slog.Info("networks subs exititing", "network", network.Name)
}

func checkin(ctx context.Context, wg *sync.WaitGroup, nc *nats.Conn, self plexus.Device, end chan bool) {
	defer wg.Done()
	log.Println("checking starting")
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("checkin done")
			return
		case <-end:
			slog.Info("ending checkin")
			nc.Close()
			return
		case <-ticker.C:
			if !nc.IsConnected() {
				log.Println("not connected to nats server, skipping checkin")
				continue
			}
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
	opts := []nats.Option{nats.Name("plexus-agent " + self.Name)}
	opts = append(opts, []nats.Option{
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(c *nats.Conn, err error) {
			slog.Info("disonnected from server", "error", err)
		}),
		nats.ClosedHandler(func(c *nats.Conn) {
			slog.Info("nats connection closed")
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			slog.Info("reconnected to nats server")
		}),
		nats.ErrorHandler(func(c *nats.Conn, s *nats.Subscription, err error) {
			slog.Info("nats error", "error", err)
		}),
		nats.Nkey(pk, sign),
	}...)
	return nats.Connect("nats://"+server+":4222", opts...)
}
