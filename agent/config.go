package agent

import (
	"errors"
	"time"

	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
)

const (
	defaultWGPort       = 51820
	defaultKeepalive    = time.Second * 20
	NatsTimeout         = time.Second * 5
	checkinTime         = time.Minute * 1
	connectivityTimeout = time.Minute * 3
	networkNotMapped    = "network not mapped to server"
)

var (
	Config   Configuration
	restart  chan struct{}
	natsfail chan struct{}
	// networkMap containss the interface name and reset channel for networks
	networkMap map[string]plexus.NetMap
	serverMap  map[string]*nats.EncodedConn
	//errors
	ErrNetNotMapped = errors.New("network not mapped to server")
	ErrConnected    = errors.New("network connected")
)

type Configuration struct {
	NatsPort  int
	Verbosity string
}
