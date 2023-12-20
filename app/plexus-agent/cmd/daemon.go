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
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

// daemonCmd represents the daemon command
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "plexus-agent deamon",
	Long: `plexus-agent daemon maintains a connection to 
a plexus server for network updates.`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("daemon called")
		daemon()
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}

func daemon() {
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
	nc, err := nats.Connect(config.Server, nats.Timeout(10*time.Second))
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()
	wg.Add(2)
	go checkin(ctx, &wg, nc)
	go mq(ctx, &wg, nc)
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
			go checkin(ctx, &wg, nc)
			go mq(ctx, &wg, nc)
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
			go checkin(ctx, &wg, nc)
			go mq(ctx, &wg, nc)
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

func handleReply(m *nats.Msg) {
	fmt.Println("reply handler", string(m.Data))
	//nc.Publish(m.Reply, "reply handler")
}

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
