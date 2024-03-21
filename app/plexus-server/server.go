package main

import (
	"log/slog"
	"net/http"
	"os/exec"
	"strings"

	"github.com/devilcove/plexus"
	"github.com/gin-gonic/gin"
)

func getServer(c *gin.Context) {
	server := struct {
		LogLevel string
		Logs     []string
	}{
		LogLevel: config.Verbosity,
	}
	cmd := exec.Command("/usr/bin/journalctl", "-eu", "plexus-server", "--no-pager", "-n", "25", "-r")
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("journalctl", "error", err)
	}
	logs := string(out)
	server.Logs = strings.Split(logs, "\n")
	c.HTML(http.StatusOK, "server", server)
}

func setLogLevel(c *gin.Context) {
	config.Verbosity = c.Param("level")
	plexus.SetLogging(config.Verbosity)
	getServer(c)
}
