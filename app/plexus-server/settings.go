package main

import (
	"log"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/devilcove/plexus"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func getSettings(c *gin.Context) {
	session := sessions.Default(c)
	page := getPage(session.Get("user"))
	c.HTML(http.StatusOK, "settings", page)
}

func updateSettings(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user").(string)
	config := plexus.Settings{}
	if err := c.Bind(&config); err != nil {
		log.Println("failed to read config", err)
		processError(c, http.StatusBadRequest, "invalid config")
		return
	}
	slog.Debug("setConfig", "config", config)
	setTheme(user, config.Theme)
	setFont(user, config.Font)
	setRefresh(user, config.Refresh)
	setPage(user, "settings")
	location := url.URL{Path: "/"}
	c.Redirect(http.StatusFound, location.RequestURI())
}
