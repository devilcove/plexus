package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/devilcove/boltdb"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("Error loading .env file")
	}
	verbosity, ok := os.LookupEnv("VERBOSITY")
	if !ok {
		verbosity = "INFO"
	}
	logger := setLogging(verbosity)
	dbfile := os.Getenv("DB_FILE")
	if dbfile == "" {
		dbfile = "time.db"
	}
	if err := boltdb.Initialize(dbfile, []string{"users", "keys", "networks", "peers", "settings"}); err != nil {
		slog.Error("database initialization", "error", err)
		os.Exit(1)
	}
	defer boltdb.Close()
	checkDefaultUser()
	wg := sync.WaitGroup{}
	quit := make(chan os.Signal, 1)
	reset := make(chan os.Signal, 1)
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
		}
	}
}

func setLogging(v string) *slog.Logger {
	logLevel := &slog.LevelVar{}
	replace := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.SourceKey {
			source, ok := a.Value.Any().(*slog.Source)
			if ok {
				source.File = filepath.Base(source.File)
				source.Function = filepath.Base(source.Function)
			}
		}
		return a
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{AddSource: true, ReplaceAttr: replace, Level: logLevel}))
	//logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{AddSource: true, Level: logLevel}))
	slog.SetDefault(logger)
	switch v {
	case "DEBUG":
		logLevel.Set(slog.LevelDebug)
	case "INFO":
		logLevel.Set(slog.LevelInfo)
	case "WARN":
		logLevel.Set(slog.LevelWarn)
	case "ERROR":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}
	if os.Getenv("DEBUG") == "true" {
		logLevel.Set(slog.LevelDebug)
	}
	slog.Info("Logging level set to", "level", logLevel.Level())
	return logger
}

func web(ctx context.Context, wg *sync.WaitGroup, logger *slog.Logger) {
	defer wg.Done()
	slog.Info("Starting web server...")
	router := setupRouter()
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}
	server := http.Server{
		Addr:    ":" + port,
		Handler: router,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http", "error", err.Error())
		}
	}()
	<-ctx.Done()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("http server shutdown", "error", err.Error())
	}
}
