package main

import (
	"crypto/rand"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

//go:embed images/* assets/* html/*
var f embed.FS

func setupRouter(logger *slog.Logger) *gin.Engine {
	authKey, encryptKey, err := sessionKeys()
	if err != nil {
		slog.Error("failed to generate session keys ..... using fallback")
		authKey = []byte("12345678901234567890123456789012")
		encryptKey = []byte("12345678901234567890123456789012")
	}
	store := cookie.NewStore(authKey, encryptKey)
	session := sessions.Sessions("plexus", store)
	if config.Verbosity != "DEBUG" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	templates := template.Must(template.ParseFS(f, "html/*"))
	router.SetHTMLTemplate(templates)
	router.GET("/images/*filepath", func(c *gin.Context) {
		c.FileFromFS(c.Request.URL.Path, http.FS(f))
	})
	router.GET("/assets/*filepath", func(c *gin.Context) {
		c.FileFromFS(c.Request.URL.Path, http.FS(f))
	})
	_ = router.SetTrustedProxies(nil)
	config := sloggin.Config{
		DefaultLevel:     slog.LevelDebug,
		ClientErrorLevel: slog.LevelWarn,
		ServerErrorLevel: slog.LevelError,
	}
	router.Use(gin.Recovery(), session, sloggin.NewWithConfig(logger, config))
	router.GET("/", displayMain)
	router.POST("/", login)
	router.GET("/logout", logout)
	sidebar := router.Group("/sidebar", auth)
	{
		sidebar.GET("/", networksSideBar)
	}
	networks := router.Group("/networks", auth)
	{
		networks.GET("/add", displayAddNetwork)
		networks.POST("/add", addNetwork)
		networks.GET("/", displayNetworks)
		networks.GET("/:id", networkDetails)
		networks.POST("/addPeer/:id/:peer", networkAddPeer)
		networks.DELETE("/:id", deleteNetwork)
		networks.DELETE("/peers/:id/:peer", removePeerFromNetwork)
		networks.GET("/relay/:id/:peer", displayAddRelay)
		networks.POST("/relay/:id/:peer", addRelay)
		networks.DELETE("/relay/:id/:peer", deleteRelay)
		networks.GET("/peers/:id/:peer", networkPeerDetails)
		networks.GET("/router/:id/:peer", displayAddRouter)
		networks.POST("/router/:id/:peer", addRouter)
		networks.DELETE("/router/:id/:peer", deleteRouter)

	}
	keys := router.Group("/keys", auth)
	{
		keys.GET("/", displayKeys)
		keys.GET("/add", displayCreateKey)
		keys.POST("/add", addKey)
		keys.DELETE("/:id", deleteKey)

	}
	peers := router.Group("/peers", auth)
	{
		peers.GET("/", displayPeers)
		peers.GET("/:id", peerDetails)
		peers.DELETE("/:id", deletePeer)
	}
	users := router.Group("/users", auth)
	{
		users.GET("/", getUsers)
		users.GET("/add", displayAddUser)
		users.POST("/add", addUser)
		users.DELETE(":name", deleteUser)
		users.GET("/user/:name", getUser)
		users.POST("user/:name", editUser)
	}
	settings := router.Group("/settings", auth)
	{
		settings.GET("/", getSettings)
		settings.POST("/logs/:level", updateSettings)
	}
	server := router.Group("/server", auth)
	{
		server.GET("/", getServer)
		server.POST("/logs/:level", setLogLevel)
	}
	return router
}

func processError(c *gin.Context, status int, message string) {
	pc, fn, line, _ := runtime.Caller(1)
	source := fmt.Sprintf("%s[%s:%d]", runtime.FuncForPC(pc).Name(), filepath.Base(fn), line)
	slog.Error(message, "status", status, "source", source)
	c.HTML(status, "error", message)
	c.Abort()
}

func auth(c *gin.Context) {
	session := sessions.Default(c)
	loggedIn := session.Get("loggedin")
	if loggedIn == nil {
		slog.Info("not logged in display login page")
		page := getPage(nil)
		page.NeedsLogin = true
		c.HTML(http.StatusOK, "login", page)
		c.Abort()
		return
	}
}

func sessionKeys() ([]byte, []byte, error) {
	authKey := make([]byte, 32)
	encryptKey := make([]byte, 32)
	_, err := rand.Read(authKey)
	if err != nil {
		return authKey, encryptKey, err
	}
	_, err = rand.Read(encryptKey)
	if err != nil {
		return authKey, encryptKey, err
	}
	return authKey, encryptKey, nil
}
