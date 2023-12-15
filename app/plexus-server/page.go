package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// var page Page
var pages map[string]Page

type Page struct {
	NeedsLogin  bool
	Version     string
	Theme       string
	Font        string
	Refresh     int
	DefaultDate string
}

func init() {
	pages = make(map[string]Page)
}

func displayMain(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user")
	loggedin := session.Get("loggedin")
	slog.Debug("display main page", "user", user, "loggedin", loggedin)
	page := getPage(user)
	c.HTML(http.StatusOK, "layout", page)
}

func initialize() Page {
	return Page{
		Version:     "v0.1.0",
		Theme:       "indigo",
		Font:        "Roboto",
		Refresh:     5,
		DefaultDate: time.Now().Local().Format("2006-01-02"),
	}
}

func getPage(user any) Page {
	if user == nil {
		return initialize()
	}
	if page, ok := pages[user.(string)]; ok {
		return page
	}
	pages[user.(string)] = initialize()
	return pages[user.(string)]
}

func SetTheme(user, theme string) {
	page, ok := pages[user]
	if !ok {
		page = initialize()
	}
	page.Theme = theme
	pages[user] = page
}

func SetFont(user, font string) {
	page, ok := pages[user]
	if !ok {
		page = initialize()
	}
	page.Font = font
	pages[user] = page
}

func SetRefresh(user string, refresh int) {
	page, ok := pages[user]
	if !ok {
		page = initialize()
	}
	page.Refresh = refresh
	pages[user] = page
}
