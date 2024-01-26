package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/spf13/viper"
)

type configuration struct {
	Admin     admin
	Server    plexusServer
	Verbosity string
	DB        db
}

type admin struct {
	Name string
	Pass string
}

type plexusServer struct {
	FQDN   string
	Secure bool
	Port   string
	Email  string
}

type db struct {
	Path   string
	File   string
	Tables []string
}

var (
	config       configuration
	ErrServerURL = errors.New("invalid server URL")
)

func configureServer() (*slog.Logger, error) {
	viper.SetDefault("admin.name", "admin")
	viper.SetDefault("admin.pass", "password")
	viper.SetDefault("verbosity", "INFO")
	viper.SetDefault("server.fqdn", "localhost")
	viper.SetDefault("server.secure", "false")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("db.file", "plexus-server.db")
	viper.SetDefault("db.tables", "[users,keys,networks,peers,settings]")
	viper.SetDefault("db.path", os.Getenv("HOME")+"/.local/share/plexus/")
	viper.SetConfigFile(os.Getenv("HOME") + "/.config/plexus-server/config")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	viper.SetEnvPrefix("PLEXUS")
	viper.AutomaticEnv()
	viper.Unmarshal(&config)

	logger := plexus.SetLogging(config.Verbosity)
	if config.Server.Secure && config.Server.FQDN == "localhost" {
		return logger, errors.New("secure server requires FQDN")
	}
	if config.Server.Secure && config.Server.Email == "" {
		return logger, errors.New("email address required")
	}
	// initalize database
	if err := os.MkdirAll(config.DB.Path, os.ModePerm); err != nil {
		return logger, err
	}
	if err := boltdb.Initialize(config.DB.Path+config.DB.File, config.DB.Tables); err != nil {
		return logger, fmt.Errorf("init database %w", err)
	}
	//check default user exists
	if err := checkDefaultUser(config.Admin.Name, config.Admin.Pass); err != nil {
		return logger, err
	}
	return logger, nil
}
