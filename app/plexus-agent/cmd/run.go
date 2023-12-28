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
	conn, err := connectToServer()
	cobra.CheckErr(err)
	ctx, cancel := context.WithCancel(context.Background())
	defer conn.Close()
	wg.Add(2)
	go checkin(ctx, &wg, conn)
	go mq(ctx, &wg, conn)
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
			wg.Add(2)
			ctx, cancel = context.WithCancel(context.Background())
			go checkin(ctx, &wg, conn)
			go mq(ctx, &wg, conn)
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
			wg.Add(2)
			ctx, cancel = context.WithCancel(context.Background())
			go checkin(ctx, &wg, conn)
			go mq(ctx, &wg, conn)
		}
	}
}

func mq(ctx context.Context, wg *sync.WaitGroup, nc *nats.Conn) {
	log.Println("mq starting")
	log.Println("config", config)
	defer wg.Done()
	sub, err := nc.Subscribe("hello", handleHello)
	if err != nil {
		log.Println("hello sub", err)
	}
	<-ctx.Done()
	log.Println("mq shutting down")
	sub.Drain()
}

//func handleReply(m *nats.Msg) {
//	fmt.Println("reply handler", string(m.Data))
//	conn.Publish(m.Reply, "reply handler")
//}

func handleHello(m *nats.Msg) {
	fmt.Println("hello subscription", string(m.Data))
	m.Respond([]byte("hello handler"))
}

func checkin(ctx context.Context, wg *sync.WaitGroup, nc *nats.Conn) {
	log.Println("checking starting")
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			log.Println("checkin done")
			return
		case <-ticker.C:
			msg, err := nc.Request("login.checkin", []byte("checking"), time.Second)
			if err != nil {
				log.Println("error publishing checkin ", err)
				continue
			}
			log.Println("checkin response", string(msg.Data))
		}
	}
}

func connectToServer() (*nats.Conn, error) {
	home := os.Getenv("HOME")
	dbfile, ok := os.LookupEnv("DB_FILE")
	if !ok {
		dbfile = home + "/.local/share/plexus/plexus-agent.db"
	}
	err := boltdb.Initialize(dbfile, []string{"devices"})
	if err != nil {
		return nil, err
	}
	defer boltdb.Close()
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		return nil, err
	}

	kp, err := nkeys.FromSeed([]byte(self.Seed))
	cobra.CheckErr(err)
	pk, err := kp.PublicKey()
	cobra.CheckErr(err)
	sign := func(nonce []byte) ([]byte, error) {
		return kp.Sign(nonce)
	}
	opts := nats.Options{
		Url:         "nats://localhost:4222",
		Nkey:        pk,
		SignatureCB: sign,
	}
	return opts.Connect()
}