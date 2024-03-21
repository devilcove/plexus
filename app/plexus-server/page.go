package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// var page Page
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

func displayMain(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	loggedin := session.Get("loggedin")
	page := getPage(user)
	if loggedin == nil {
		page.NeedsLogin = true
	}
	networks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		slog.Error("get networks for main display", "error", err)
	}
	page.Data = networks
	page.Page = "networks"
	slog.Debug("display main page", "user", user, "page", page)
	c.HTML(http.StatusOK, "layout", page)
}

func login(c *gin.Context) {
	session := sessions.Default(c)
	var user plexus.User
	if err := c.Bind(&user); err != nil {
		processError(c, http.StatusBadRequest, "invalid user")
		slog.Error("bind err", "error", err)
		return
	}
	slog.Info("login by", "user", user)
	if !validateUser(&user) {
		session.Clear()
		_ = session.Save()
		processError(c, http.StatusBadRequest, "invalid user")
		slog.Warn("validation error", "user", user.Username)
		return
	}
	session.Set("loggedin", true)
	session.Set("user", user.Username)
	session.Set("admin", user.IsAdmin)
	session.Set("page", "networks")
	session.Options(sessions.Options{MaxAge: sessionAge, Secure: false, SameSite: http.SameSiteLaxMode})
	_ = session.Save()
	slog.Info("login", "user", user.Username)
	page := getPage(user.Username)
	page.NeedsLogin = false
	page.Page = "networks"
	displayMain(c)
	//c.HTML(http.StatusOK, "content", page)
}

func validateUser(visitor *plexus.User) bool {
	user, err := boltdb.Get[plexus.User](visitor.Username, userTable)
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

func logout(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	slog.Info("logout", "user", user)
	//delete cookie
	session.Clear()
	_ = session.Save()
	c.HTML(http.StatusOK, "login", "")
}

func initialize() Page {
	networks := []string{}
	allNetworks, err := boltdb.GetAll[plexus.Network](networkTable)
	if err != nil {
		slog.Error("get networks during page init", "error", err)
	}
	for _, network := range allNetworks {
		networks = append(networks, network.Name)

	}
	return Page{
		Version:     "v0.1.0",
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
		networks, err := boltdb.GetAll[plexus.Network](networkTable)
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

func setPage(user any, pageToSet string) {
	log.Println("setting page", pageToSet, " for user", user)
	if user == nil {
		return
	}
	page, ok := pages[user.(string)]
	if !ok {
		page = initialize()
	}
	page.Page = pageToSet
	pages[user.(string)] = page
}

func setTheme(user, theme string) {
	page, ok := pages[user]
	if !ok {
		page = initialize()
	}
	page.Theme = theme
	pages[user] = page
}

func setFont(user, font string) {
	page, ok := pages[user]
	if !ok {
		page = initialize()
	}
	page.Font = font
	pages[user] = page
}

func setRefresh(user string, refresh int) {
	page, ok := pages[user]
	if !ok {
		page = initialize()
	}
	page.Refresh = refresh
	pages[user] = page
}
