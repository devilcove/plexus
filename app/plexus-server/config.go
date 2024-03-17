package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/mail"
	"os"
	"strings"
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

const (
	userTable    = "users"
	keyTable     = "keys"
	networkTable = "networks"
	peerTable    = "peers"
	settingTable = "settings"
)

var (
	config       configuration
	ErrServerURL = errors.New("invalid server URL")
	sessionAge   = 60 * 60 * 24
	version      = "v0.1.0"
	path         = "/var/lib/plexus/"
)

const (
	//timers
	connectedTime = time.Second * 30
	natsTimeout   = time.Second * 3
	keyExpiry     = time.Hour * 24
	keyTick       = time.Hour * 6
	pingTick      = time.Minute * 3
)

func configureServer() (*slog.Logger, error) {
	viper.SetDefault("adminname", "admin")
	viper.SetDefault("adminpass", "password")
	viper.SetDefault("verbosity", "INFO")
	//viper.SetDefault("fqdn", "localhost")
	viper.SetDefault("secure", true)
	viper.SetDefault("port", "8080")
	viper.SetDefault("email", "")
	viper.SetDefault("dbfile", "plexus-server.db")
	viper.SetDefault("tables", []string{userTable, keyTable, networkTable, peerTable, settingTable})
	viper.SetDefault("dbpath", path)
	viper.SetConfigFile("/etc/plexus/config")
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
	if config.Secure {
		if config.FQDN == "" {
			return logger, errors.New("secure server requires FQDN")
		}
		if net.ParseIP(config.FQDN) != nil {
			return logger, errors.New("cannot use IP address with secure")
		}
		if !emailValid(config.Email) {
			return logger, errors.New("valid email address required")
		}

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

func emailValid(email string) bool {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	if strings.Contains(email, "example.com") {
		return false
	}
	return true
}
