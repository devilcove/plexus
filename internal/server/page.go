package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/devilcove/cookie"
	"github.com/devilcove/plexus"
	"golang.org/x/crypto/bcrypt"
)

// var page Page.
var (
	pages map[string]Page
)

type Page struct {
	Page        string
	NeedsLogin  bool
	Version     string
	Theme       string
	Font        string
	Refresh     int
	DefaultDate string
	Networks    []string
	Data        any
}

func init() {
	pages = make(map[string]Page)
}

func displayMain(w http.ResponseWriter, r *http.Request) {
	page := initialize()
	page.NeedsLogin = true
	if _, err := cookie.Get(r, cookieName); err != nil {
		page.NeedsLogin = true
	}
	networks, err := store.GetAll[plexus.Network](networkBucket)
	if err != nil {
		slog.Error("get networks for main display", "error", err)
	}
	page.Data = networks

	render(w, "layout", page)
}

func login(w http.ResponseWriter, r *http.Request) {
	var user plexus.User
	user.Username = r.FormValue("username")
	user.Password = r.FormValue("password")

	if !validateUser(&user) {
		processError(w, http.StatusBadRequest, "invalid user")
		return
	}
	user.Password = "" // clear password.
	saveSession(w, user)

	slog.Debug("login", "user", user.Username)
	page := getPage(user.Username)
	page.NeedsLogin = false
	page.Page = "networks"
	saveSession(w, user)
	render(w, "layout", page)
}

func validateUser(visitor *plexus.User) bool {
	user, err := store.Get[plexus.User](visitor.Username, userBucket)
	if err != nil {
		slog.Error("no such user", "user", visitor.Username, "error", err)
		return false
	}
	if visitor.Username == user.Username && checkPassword(visitor, &user) {
		visitor.IsAdmin = user.IsAdmin
		return true
	}
	return false
}

func checkPassword(plain, hash *plexus.User) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash.Password), []byte(plain.Password))
	if err != nil {
		slog.Debug("bcrypt", "error", err)
	}
	return err == nil
}

func logout(w http.ResponseWriter, r *http.Request) {
	ClearSession(w)
	slog.Debug("logout")
	http.Redirect(w, r, "/", http.StatusFound)
}

func initialize() Page {
	networks := []string{}
	allNetworks, err := store.GetAll[plexus.Network](networkBucket)
	if err != nil {
		slog.Error("get networks during page init", "error", err)
	}
	for _, network := range allNetworks {
		networks = append(networks, network.Name)
	}
	info, _ := debug.ReadBuildInfo()
	return Page{
		Version:     info.Main.Version,
		Theme:       "black",
		Font:        "PT Sans",
		Refresh:     5,
		DefaultDate: time.Now().Local().Format("2006-01-02"),
		Page:        "networks",
		Networks:    networks,
		Data:        allNetworks,
	}
}

func getPage(user any) Page {
	if user == nil {
		return initialize()
	}
	if page, ok := pages[user.(string)]; ok {
		page.DefaultDate = time.Now().Local().Format("2006-01-02")
		networks, err := store.GetAll[plexus.Network](networkBucket)
		if err != nil {
			slog.Error("get networks", "error", err)
		}
		page.Networks = []string{}
		for _, net := range networks {
			page.Networks = append(page.Networks, net.Name)
		}
		return page
	}
	pages[user.(string)] = initialize()
	return pages[user.(string)]
}

func render(w io.Writer, template string, data any) {
	if err := templates.ExecuteTemplate(w, template, data); err != nil {
		slog.Error("render template", "caller", caller(2),
			"name", template, "data", data, "error", err)
	}
}

func caller(depth int) string {
	pc, file, no, ok := runtime.Caller(depth)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		return fmt.Sprintf("%s %s:%d", details.Name(), filepath.Base(file), no)
	}
	return "unknown caller"
}
