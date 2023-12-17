package main

import (
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/database"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

//go:embed: images/favicon.ico
var icon embed.FS

func setupRouter() *gin.Engine {
	//gin.SetMode(gin.ReleaseMode)
	secret, ok := os.LookupEnv("SESSION_SECRET")
	if !ok {
		secret = "secret"
	}
	store := cookie.NewStore([]byte(secret))
	session := sessions.Sessions("plexus", store)
	router := gin.Default()
	router.LoadHTMLGlob("html/*.html")
	router.Static("images", "./images")
	router.Static("assets", "./assets")
	router.StaticFS("/favicon.ico", http.FS(icon))
	//router.SetHTMLTemplate(template.Must(template.New("").Parse("html/*")))
	_ = router.SetTrustedProxies(nil)
	router.Use(gin.Recovery(), session)
	router.GET("/", displayMain)
	router.POST("/login", login)
	router.GET("/logout", logout)
	networks := router.Group("networks", auth)
	{
		networks.GET("/add", displayAddNetwork)
		networks.POST("add", addNetwork)
		networks.GET("/:id", displayNetwork)
	}
	//router.GET("/login", displayLogin)
	//users := router.Group("/users", auth)
	//{
	//	users.GET("", getUsers)
	//	users.GET("current", getUser)
	//	users.POST("", addUser)
	//	users.POST(":name", editUser)
	//	users.DELETE(":name", deleteUser)
	//	users.GET(":name", getUser)
	//}
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
	configuration := router.Group("/config", auth)
	{
		configuration.GET("/", config)
		configuration.POST("/", setConfig)
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

func checkDefaultUser() {
	if database.AdminExist() {
		slog.Debug("admin exists")
		return
	}
	user, ok := os.LookupEnv("PLEXUS_USER")
	if !ok {
		user = "admin"
	}
	pass, ok := os.LookupEnv("PLEXUS_PASS")
	if !ok {
		pass = "password"
	}
	password, err := database.HashPassword(pass)
	if err != nil {
		slog.Error("hash error", "error", err)
		return
	}
	if err = database.SaveUser(&plexus.User{
		Username: user,
		Password: password,
		IsAdmin:  true,
		Updated:  time.Now(),
	}); err != nil {
		slog.Error("create default user", "error", err)
		return
	}
	slog.Info("default user created")
}
