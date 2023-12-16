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
	"time"

	"github.com/devilcove/plexus/database"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/kr/pretty"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
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
	database.InitializeDatabase()
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

func broker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	slog.Info("Starting broker...")
	opts := &server.Options{}
	ns, err := server.NewServer(opts)
	if err != nil {
		slog.Error("nats server", "error", err)
		return
	}
	go ns.Start()
	if !ns.ReadyForConnections(3 * time.Second) {
		slog.Error("not ready for connection", "error", err)
		return
	}
	nc, err := nats.Connect("localhost:4222")
	if err != nil {
		slog.Error("nats connect", "error", err)
	}
	if _, err := nc.Subscribe("login.*", loginReply); err != nil {
		slog.Error("subscribe login", "error", err)
	}
	wg.Add(1)
	<-ctx.Done()
}

func loginReply(msg *nats.Msg) {
	start := time.Now()
	name := msg.Subject[6:]
	slog.Info("login message", "name", name)
	pretty.Println("header", msg.Header)
	pretty.Println("repy", msg.Reply)
	pretty.Println("subject", msg.Subject)
	pretty.Println("data", string(msg.Data))
	pretty.Println("sub", msg.Sub.Queue, msg.Sub.Subject)
	msg.Respond([]byte("hello, " + name))
	slog.Info("login reply", "duration", time.Since(start))
}

func login(c *gin.Context) {
	start := time.Now()
	type User struct {
		Name string `json:"name"`
	}
	var user User
	if err := c.BindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		c.Abort()
		return
	}
	slog.Info("login", "name", user.Name)
	//pretty.Println("header", c.Request.Header)
	//pretty.Println("body", c.Request.Body)
	c.String(http.StatusOK, "hello, %s", user.Name)
	slog.Info("login", "duration", time.Since(start))
}
