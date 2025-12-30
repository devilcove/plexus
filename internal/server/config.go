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
	"github.com/devilcove/configuration"
	"github.com/devilcove/plexus"
)

type Configuration struct {
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
	ErrServerURL       = errors.New("invalid server URL")
	ErrInvalidSubnet   = errors.New("invalid subnet")
	ErrSubnetInUse     = errors.New("subnet in use")
	ErrDataDir         = errors.New("data dir not found")
	ErrSecureBlankFQDN = errors.New("secure server requires FQDN")
	ErrSecureWithIP    = errors.New("cannot use IP address with secure")
	ErrInValidEmail    = errors.New("valid email address required")
	version            = "v0.3.0"
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
	config := Configuration{}
	if err := configuration.Get(&config); err != nil {
		return nil, err
	}
	// set defaults
	if config.AdminName == "" {
		config.AdminName = "admin"
	}
	if config.AdminPass == "" {
		config.AdminPass = "password"
	}
	if config.Verbosity == "" {
		config.Verbosity = "INFO"
	}
	if config.Port == "" {
		config.Port = "8080"
	}
	if config.DBFile == "" {
		config.DBFile = "plexus-server.db"
	}
	if config.DataHome == "" {
		config.DataHome = home + "/.local/share/" + filepath.Base(os.Args[0]) + "/"
	}
	if _, err := os.Stat(config.DataHome); err != nil {
		return nil, ErrDataDir
	}

	slog.Info("configure Server", "config", config)
	var tlsConfig *tls.Config
	plexus.SetLogging(config.Verbosity)
	if config.Secure {
		if config.FQDN == "" {
			return nil, ErrSecureBlankFQDN
		}
		if net.ParseIP(config.FQDN) != nil {
			return nil, ErrSecureWithIP
		}
		if !emailValid(config.Email) {
			return nil, ErrInValidEmail
		}
	}
	// initialize database.
	if err := os.MkdirAll(config.DataHome, os.ModePerm); err != nil {
		return nil, err
	}
	slog.Info("init db", "path", config.DataHome, "file", config.DBFile)
	if err := boltdb.Initialize(
		filepath.Join(config.DataHome, config.DBFile),
		[]string{"users", "keys", "networks", "peers", "settings"},
	); err != nil {
		return nil, fmt.Errorf("init database %w", err)
	}
	// check default user exists.
	if err := checkDefaultUser(config.AdminName, config.AdminPass); err != nil {
		return nil, err
	}
	// get TLS.
	if config.Secure {
		var err error
		certmagic.DefaultACME.Agreed = true
		certmagic.DefaultACME.Email = config.Email
		certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
		tlsConfig, err = certmagic.TLS([]string{config.FQDN})
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
