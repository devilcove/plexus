package agent

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	defaultWGPort       = 51820
	maxNetworks         = 100
	defaultKeepalive    = time.Second * 20
	NatsTimeout         = time.Second * 5
	NatsLongTimeout     = time.Second * 15
	checkinTime         = time.Minute * 1
	serverCheckTime     = time.Minute * 1
	connectivityTimeout = time.Minute * 3
	networkNotMapped    = "network not mapped to server"
	version             = "v0.1.0"
	networkTable        = "networks"
	deviceTable         = "devices"
	path                = "/var/lib/plexus/"
)

var (
	Config        Configuration
	serverConn    atomic.Pointer[nats.EncodedConn]
	subscriptions []*nats.Subscription
	//errors
	ErrNetNotMapped = errors.New("network not mapped to server")
	ErrConnected    = errors.New("network connected")
)

type Configuration struct {
	NatsPort  int
	Verbosity string
}
