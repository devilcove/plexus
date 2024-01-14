package main

import (
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/joho/godotenv"
)

type Configuration struct {
	Admin        string
	AdminPass    string
	Verbosity    string
	Server       string
	DatabaseFile string
	Tables       []string
}

var (
	config       Configuration
	ErrServerURL = errors.New("invalid server URL")
)

func configureServer() (*slog.Logger, error) {
	ok := false
	if err := godotenv.Load(); err != nil {
		slog.Warn("Error loading .env file")
	}
	// get verbosity and setup logging
	config.Verbosity, ok = os.LookupEnv("VERBOSITY")
	if !ok {
		config.Verbosity = "INFO"
	}
	logger := plexus.SetLogging(config.Verbosity)
	// set server URL
	config.Server, ok = os.LookupEnv("PLEXUS_URL")
	if !ok {
		config.Server = "nats://localhost:4222"
	}
	if !strings.Contains(config.Server, "nats://") {
		return logger, ErrServerURL
	}
	// initalize database
	home := os.Getenv("HOME")
	config.DatabaseFile = os.Getenv("DB_FILE")
	if config.DatabaseFile == "" {
		config.DatabaseFile = home + "/.local/share/plexus/plexus-server.db"
		if err := os.MkdirAll(home+".local/share/plexus", os.ModePerm); err != nil {
			return logger, err
		}
	}
	config.Tables = []string{"users", "keys", "networks", "peers", "settings"}
	if err := boltdb.Initialize(config.DatabaseFile, config.Tables); err != nil {
		return logger, err
	}
	// check default user exists
	config.Admin, ok = os.LookupEnv("PLEXUS_USER")
	if !ok {
		config.Admin = "admin"
	}
	config.AdminPass, ok = os.LookupEnv("PLEXUS_PASS")
	if !ok {
		config.AdminPass = "password"
	}
	if err := checkDefaultUser(config.Admin, config.AdminPass); err != nil {
		return logger, err
	}
	return logger, nil

}
