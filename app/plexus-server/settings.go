package main

import (
	"log"
	"log/slog"
	"net/http"

	"github.com/devilcove/plexus"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func config(c *gin.Context) {
	session := sessions.Default(c)
	page := getPage(session.Get("user"))
	c.HTML(http.StatusOK, "settings", page)
}

func setConfig(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get("user").(string)
	config := plexus.Config{}
	if err := c.Bind(&config); err != nil {
		log.Println("failed to read config", err)
		processError(c, http.StatusBadRequest, "invalid config")
		return
	}
	slog.Debug("setConfig", "config", config)
	SetTheme(user, config.Theme)
	SetFont(user, config.Font)
	SetRefresh(user, config.Refresh)
	displayMain(c)
}
