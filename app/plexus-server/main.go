package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/caddyserver/certmagic"
	"github.com/devilcove/boltdb"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

var (
	newDevice   chan string
	brokerfail  chan int
	webfail     chan int
	natServer   *server.Server
	natsOptions *server.Options
	natsConn    *nats.Conn
)

func main() {

	logger, err := configureServer()
	if err != nil {
		slog.Error("unable to configure server", "error", err)
		os.Exit(1)
	}
	defer boltdb.Close()
	wg := sync.WaitGroup{}
	quit := make(chan os.Signal, 1)
	reset := make(chan os.Signal, 1)
	brokerfail = make(chan int, 1)
	webfail = make(chan int, 1)
	newDevice = make(chan string, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	signal.Notify(reset, syscall.SIGHUP)
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(2)
	go web(ctx, &wg, logger)
	go broker(ctx, &wg)
	for {
		select {
		case <-quit:
			slog.Info("Shutting down...")
			cancel()
			wg.Wait()
			return
		case <-reset:
			slog.Info("reset...")
			cancel()
			wg.Wait()
			ctx, cancel = context.WithCancel(context.Background())
			wg.Add(2)
			go web(ctx, &wg, logger)
			go broker(ctx, &wg)
		case <-brokerfail:
			slog.Error("error running broker .... shutting down")
			cancel()
			wg.Wait()
			os.Exit(1)
		case <-webfail:
			slog.Error("error running web .... shutting down")
			cancel()
			wg.Wait()
			os.Exit(2)
		}
	}
}

func web(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger) {
	defer wg.Done()
	slog.Info("Starting web server...")
	router := setupRouter()
	if config.Secure {
		certmagic.DefaultACME.Agreed = true
		certmagic.DefaultACME.Email = config.Email
		certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
		go func() {
			if err := certmagic.HTTPS([]string{config.FQDN}, router); err != nil {
				slog.Error("https", "error", err)
				webfail <- 1
			}
		}()
		<-ctx.Done()
		return
	} else {
		server := http.Server{
			Addr:    ":" + config.Port,
			Handler: router,
		}
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("http", "error", err.Error())
				webfail <- 1
			}
		}()
		<-ctx.Done()
		if err := server.Shutdown(ctx); err != nil {
			slog.Error("http server shutdown", "error", err.Error())
		}
	}
}
