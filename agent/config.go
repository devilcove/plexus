package agent

import (
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
)

const defaultStart = 51820

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
