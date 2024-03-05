package main

import (
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
)

//go:embed images/* assets/* html/*
var f embed.FS

func setupRouter() *gin.Engine {
	store := cookie.NewStore([]byte(config.SessionSecret))
	session := sessions.Sessions("plexus", store)
	if config.Verbosity != "DEBUG" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	templates := template.Must(template.ParseFS(f, "html/*"))
	router.SetHTMLTemplate(templates)
	router.GET("/images/*filepath", func(c *gin.Context) {
		c.FileFromFS(c.Request.URL.Path, http.FS(f))
	})
	router.GET("/assets/*filepath", func(c *gin.Context) {
		c.FileFromFS(c.Request.URL.Path, http.FS(f))
	})
	_ = router.SetTrustedProxies(nil)
	router.Use(gin.Recovery(), session)

	router.GET("/", displayMain)
	router.POST("/", login)
	router.GET("/logout", logout)
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
	//router.GET("/login", displayLogin)
	users := router.Group("/nats", auth)
	{
		users.GET("/", getNatsUsers)
		//	users.GET("current", getUser)
		//	users.POST("", addUser)
		//	users.POST(":name", editUser)
		//	users.DELETE(":name", deleteUser)
		//	users.GET(":name", getUser)
	}
	//router.GET("/register", register)
	//router.POST("/register", regUser)
	//projects := router.Group("/projects", auth)
	//{
	//	projects.GET("", getProjects)
	//	projects.GET("/add", displayProjectForm)
	//	projects.POST("", addProject)
	//	projects.GET("/:name", getProject)
	//	projects.POST("/:name/start", start)
	//	projects.POST("/stop", stop)
	//	projects.GET("/status", displayStatus)
	//}
	//reports := router.Group("/reports", auth)
	//{
	//	reports.GET("", report)
	//	reports.POST("", getReport)
	//}
	//records := router.Group("records", auth)
	//{
	//	records.GET("/:id", getRecord)
	//	records.POST("/:id", editRecord)
	//}
	settings := router.Group("/settings", auth)
	{
		settings.GET("/", getSettings)
		settings.POST("/", updateSettings)
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
