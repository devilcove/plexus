package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/spf13/viper"
)

type configuration struct {
	AdminName string
	AdminPass string
	FQDN      string
	Secure    bool
	Port      string
	Email     string
	Verbosity string
	DBPath    string
	DBFile    string
	Tables    []string
}

var (
	config       configuration
	ErrServerURL = errors.New("invalid server URL")
	//timers
	connectedTime = time.Second * 30
)

func configureServer() (*slog.Logger, error) {
	viper.SetDefault("adminname", "admin")
	viper.SetDefault("adminpass", "password")
	viper.SetDefault("verbosity", "INFO")
	viper.SetDefault("fqdn", "localhost")
	viper.SetDefault("secure", false)
	viper.SetDefault("port", "8080")
	viper.SetDefault("email", "")
	viper.SetDefault("dbfile", "plexus-server.db")
	viper.SetDefault("tables", []string{"users", "keys", "networks", "peers", "settings"})
	viper.SetDefault("dbpath", os.Getenv("HOME")+"/.local/share/plexus/")
	viper.SetConfigFile(os.Getenv("HOME") + "/.config/plexus-server/config")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	viper.SetEnvPrefix("PLEXUS")
	viper.AutomaticEnv()
	if err := viper.UnmarshalExact(&config); err != nil {
		return nil, err
	}
	logger := plexus.SetLogging(config.Verbosity)
	if config.Secure && config.FQDN == "localhost" {
		return logger, errors.New("secure server requires FQDN")
	}
	if config.Secure && config.Email == "" {
		return logger, errors.New("email address required")
	}
	// initalize database
	if err := os.MkdirAll(config.DBPath, os.ModePerm); err != nil {
		return logger, err
	}
	slog.Info("init db", "path", config.DBFile, "file", config.DBFile, "tables", config.Tables)
	if err := boltdb.Initialize(config.DBPath+config.DBFile, config.Tables); err != nil {
		return logger, fmt.Errorf("init database %w", err)
	}
	//check default user exists
	if err := checkDefaultUser(config.AdminName, config.AdminPass); err != nil {
		return logger, err
	}
	return logger, nil
}
