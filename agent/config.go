package agent

import (
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
)

var (
	Config   Configuration
	restart  chan struct{}
	natsfail chan struct{}
	// networkMap containss the interface name and reset channel for networks
	networkMap map[string]plexus.NetMap
	serverMap  map[string]*nats.EncodedConn
)

type Configuration struct {
	NatsPort  int
	Verbosity string
}
