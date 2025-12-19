package server

import (
	"bytes"
	"crypto/rand"
	"embed"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/devilcove/mux"
)

//go:embed images/* assets/* html/*
var f embed.FS

const htmlFiles = "/home/mkasun/sandbox/plexus/internal/server/html/*"

var (
	logger    *slog.Logger
	templates *template.Template
)

func setupRouter(l *slog.Logger) *mux.Router {
	InitializeSession()

	router := mux.NewRouter(l, mux.Logger)
	logger = l
	dir, _ := os.Getwd()
	slog.Info("here", "pwd", dir)
	templates = template.Must(template.ParseGlob(htmlFiles))

	// static files
	router.StaticFS("/images/", f)
	router.StaticFS("/assets/", f)
	router.ServeFileFS("/favicon.ico", "icon.svg", f)

	// unauthorized routes
	router.Post("/login/", login)
	router.Get("/logout/", logout)
	router.Get("/{$}", displayMain)

	sidebar := router.Group("/sidebar", auth)
	sidebar.Get("/", networksSideBar)

	networks := router.Group("/networks", auth)
	networks.Get("/add", displayAddNetwork)
	networks.Post("/add", addNetwork)
	networks.Get("/", displayNetworks)
	networks.Get("/{id}", networkDetails)
	networks.Post("/addPeer/{id}/{peer}", networkAddPeer)
	networks.Delete("/{id}", deleteNetwork)
	networks.Delete("/peers/{id}/{peer}", removePeerFromNetwork)
	networks.Get("/relay/{id}/{peer}", displayAddRelay)
	networks.Post("/relay/{id}/{peer}", addRelay)
	networks.Delete("/relay/{id}/{peer}", deleteRelay)
	networks.Get("/peers/{id}/{peer}", networkPeerDetails)
	networks.Get("/router/{id}/{peer}", displayAddRouter)
	networks.Post("/router/{id}/{peer}", addRouter)
	networks.Delete("/router/{id}/{peer}", deleteRouter)

	keys := router.Group("/keys", auth)
	keys.Get("/", displayKeys)
	keys.Get("/add", displayCreateKey)
	keys.Post("/add", addKey)
	keys.Delete("/{id}", deleteKey)

	peers := router.Group("/peers", auth)
	peers.Get("/{$}", displayPeers)
	peers.Get("/{id}", peerDetails)
	peers.Delete("/{id}", deletePeer)

	users := router.Group("/users", auth)
	users.Get("/{$}", getUsers)
	users.Get("/add", displayAddUser)
	users.Post("/add", addUser)
	users.Delete("/{name}", deleteUser)
	users.Get("/user/{name}", getUser)
	users.Post("user/{name}", editUser)

	server := router.Group("/server", auth)
	server.Get("/", getServer)
	server.Post("/logs/{level}", setLogLevel)

	return router
}

func processError(w http.ResponseWriter, status int, message string) {
	buf := bytes.Buffer{}
	l := log.New(&buf, "", log.Lshortfile)
	_ = l.Output(2, message)
	slog.Error(buf.String())
	http.Error(w, message, status)
}

func auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(w, r)
		if session == nil {
			displayLogin(w, r)
			return
		}
		if !session.LoggedIn {
			displayLogin(w, r)
			return
		}
		if err := session.Session.Save(r, w); err != nil {
			slog.Error("save session", "error", err)
		}
		next.ServeHTTP(w, r)
	})
}

// func auth(c *gin.Context) {
// 	session := sessions.Default(c)
// 	loggedIn := session.Get("loggedin")
// 	if loggedIn == nil {
// 		slog.Info("not logged in display login page")
// 		page := getPage(nil)
// 		page.NeedsLogin = true
// 		if err := templates.ExecuteTemplate(w, "login", page)
// 		c.Abort()
// 		return
// 	}
// }

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

// func weblogger() gin.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		start := time.Now()
// 		c.Next()
// 		status := c.Writer.Status()
// 		switch {
// 		case status >= 500:
// 			slog.Error("request", "Code", c.Writer.Status(), "method", c.Request.Method,
// 				"route", c.Request.URL.Path, "latency", time.Since(start), "client", c.ClientIP())
// 		case status >= 400:
// 			slog.Warn("request", "Code", c.Writer.Status(), "method", c.Request.Method,
// 				"route", c.Request.URL.Path, "latency", time.Since(start), "client", c.ClientIP())
// 		default:
// 			slog.Debug("request", "Code", c.Writer.Status(), "method", c.Request.Method,
// 				"route", c.Request.URL.Path, "latency", time.Since(start), "client", c.ClientIP())
// 		}
// 	}
// }
