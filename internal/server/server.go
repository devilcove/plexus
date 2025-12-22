package server

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/mattkasun/tools/logging"
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
	// eConn       *nats.EncodedConn.
)

// Run - run the server.
func Run() {
	tlsConfig, err := configureServer()
	if err != nil {
		slog.Error("unable to configure server", "error", err)
		os.Exit(1)
	}
	// defer boltdb.Close()
	wg := sync.WaitGroup{}
	quit := make(chan os.Signal, 1)
	reset := make(chan os.Signal, 1)
	brokerfail = make(chan int, 1)
	webfail = make(chan int, 1)
	newDevice = make(chan string, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	signal.Notify(reset, syscall.SIGHUP)
	ctx, cancel := context.WithCancel(context.Background())
	start(ctx, &wg, tlsConfig)
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
			start(ctx, &wg, tlsConfig)
		case <-brokerfail:
			slog.Error("error running broker .... shutting down")
			cancel()
			wg.Wait()
			boltdb.Close()
			os.Exit(1)
		case <-webfail:
			slog.Error("error running web .... shutting down")
			cancel()
			wg.Wait()
			boltdb.Close()
			os.Exit(2)
		}
	}
}

func start(ctx context.Context, wg *sync.WaitGroup, tls *tls.Config) {
	wg.Add(2)
	go web(ctx, wg, tls)
	go broker(ctx, wg, tls)
}

func web(ctx context.Context, wg *sync.WaitGroup, tls *tls.Config) {
	defer wg.Done()
	slog.Info("Starting web server...")
	router := setupRouter(logging.TextLogger(logging.TruncateSource(), logging.TimeFormat(time.DateTime)).Logger)
	// server := http.Server{
	// 	Addr:    ":" + config.Port,
	// 	Handler: router,
	// }
	// if config.Secure {
	// 	if tls == nil {
	// 		slog.Error("secure set but tls nil")
	// 		webfail <- 1
	// 	}
	// 	server.TLSConfig = tls
	// 	server.Addr = ":443"
	// 	go func() {
	// 		if err := server.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
	// 			slog.Error("https server", "error", err)
	// 			webfail <- 1
	// 		}
	// 	}()
	// } else {
	// 	go func() {
	// 		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
	// 			slog.Error("http server", "error", err)
	// 			webfail <- 1
	// 		}
	// 	}()
	// }

	go func() {
		router.Run(":8090")
	}()
	slog.Info("web server started")
	<-ctx.Done()
	slog.Info("shutting down web server")
	//if err := server.Shutdown(ctx); err != nil {
	//slog.Error("http server shutdown", "error", err.Error())
	//}
	slog.Info("http server shutdown")
}

func getServer(w http.ResponseWriter, r *http.Request) {
	server := struct {
		LogLevel string
		Logs     []string
	}{
		LogLevel: cfg.Verbosity,
	}
	cmd := exec.Command("/usr/bin/journalctl", "-eu", "plexus-server", "--no-pager", "-n", "25", "-r")
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("journalctl", "error", err)
	}
	logs := string(out)
	server.Logs = strings.Split(logs, "\n")
	if err := templates.ExecuteTemplate(w, "server", server); err != nil {
		slog.Error("execute template", "template", "server", "data", server, "error", err)
	}
}

func setLogLevel(w http.ResponseWriter, r *http.Request) {
	cfg.Verbosity = r.PathValue("level")
	plexus.SetLogging(cfg.Verbosity)
	getServer(w, r)
}
