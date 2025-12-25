package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/mattkasun/tools/config"
)

type configuration struct {
	AdminName string
	AdminPass string
	FQDN      string
	Secure    bool
	Port      string
	Email     string
	Verbosity string
	DataHome  string
	DBFile    string
}

const (
	userTable    = "users"
	keyTable     = "keys"
	networkTable = "networks"
	peerTable    = "peers"
	settingTable = "settings"
)

var (
	cfg              *configuration
	ErrServerURL     = errors.New("invalid server URL")
	ErrInvalidSubnet = errors.New("invalid subnet")
	ErrSubnetInUse   = errors.New("subnet in use")
	version          = "v0.3.0"
)

const (
	// timers.
	connectedTime = time.Second * 30
	natsTimeout   = time.Second * 3
	keyExpiry     = time.Hour * 24
	keyTick       = time.Hour * 6
	pingTick      = time.Minute * 3
)

func configureServer() (*tls.Config, error) {
	plexus.SetLogging("INFO")
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	cfg, err = config.Get[configuration]()
	if err != nil {
		return nil, err
	}
	// set defaults
	if cfg.AdminName == "" {
		cfg.AdminName = "admin"
	}
	if cfg.AdminPass == "" {
		cfg.AdminPass = "password"
	}
	if cfg.Verbosity == "" {
		cfg.Verbosity = "INFO"
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DBFile == "" {
		cfg.DBFile = "plexus-server.db"
	}
	if cfg.DataHome == "" {
		cfg.DataHome = home + "/.local/share/" + filepath.Base(os.Args[0]) + "/"
	}
	if _, err := os.Stat(cfg.DataHome); err != nil {
		return nil, err
	}

	slog.Info("configure Server", "config", cfg)
	var tlsConfig *tls.Config
	plexus.SetLogging(cfg.Verbosity)
	if cfg.Secure {
		if cfg.FQDN == "" {
			return nil, errors.New("secure server requires FQDN")
		}
		if net.ParseIP(cfg.FQDN) != nil {
			return nil, errors.New("cannot use IP address with secure")
		}
		if !emailValid(cfg.Email) {
			return nil, errors.New("valid email address required")
		}
	}
	// initialize database.
	if err := os.MkdirAll(cfg.DataHome, os.ModePerm); err != nil {
		return nil, err
	}
	slog.Info("init db", "path", cfg.DataHome, "file", cfg.DBFile)
	if err := boltdb.Initialize(
		cfg.DataHome+cfg.DBFile,
		[]string{"users", "keys", "networks", "peers", "settings"},
	); err != nil {
		return nil, fmt.Errorf("init database %w", err)
	}
	// check default user exists.
	if err := checkDefaultUser(cfg.AdminName, cfg.AdminPass); err != nil {
		return nil, err
	}
	// get TLS.
	if cfg.Secure {
		var err error
		certmagic.DefaultACME.Agreed = true
		certmagic.DefaultACME.Email = cfg.Email
		certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
		tlsConfig, err = certmagic.TLS([]string{cfg.FQDN})
		if err != nil {
			return nil, err
		}
	}
	return tlsConfig, nil
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
