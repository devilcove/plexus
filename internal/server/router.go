package server

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/devilcove/mux"
)

//go:embed images assets html
var content embed.FS

var templates *template.Template

func setupRouter(l *slog.Logger) *mux.Router {
	InitializeSession()

	router := mux.NewRouter(l, mux.Logger)
	dir, _ := os.Getwd()
	slog.Info("here", "pwd", dir)
	templates = template.Must(template.ParseFS(content, "html/*.html"))

	// static files
	router.StaticFS("/content/", content)
	router.ServeFileFS("/favicon.ico", "images/icon.svg", content)

	// unauthorized routes
	router.Post("/login/", login)
	router.Get("/logout/", logout)
	router.Get("/{$}", displayMain)

	sidebar := router.Group("/sidebar", auth)
	sidebar.Get("/", networksSideBar)

	networks := router.Group("/networks", auth)
	networks.Get("/add", displayAddNetwork)
	networks.Post("/add", addNetwork)
	networks.Get("/{$}", displayNetworks)
	networks.Get("/details/{id}", networkDetails)
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
	users.Post("/user/{name}", editUser)

	server := router.Group("/server", auth)
	server.Get("/", getServer)
	server.Post("/logs/{level}", setLogLevel)

	return router
}

func processError(w http.ResponseWriter, status int, message string) {
	header := fmt.Sprintf(`{"showError":"%s"}`, message)
	buf := bytes.Buffer{}
	l := log.New(&buf, "", log.Lshortfile)
	_ = l.Output(2, message)
	slog.Error(buf.String())
	w.Header().Set("Hx-Trigger", header)
	http.Error(w, message, status)
}

func auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := GetSession(r)
		if session.IsNew {
			http.Redirect(w, r, "/login/", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
