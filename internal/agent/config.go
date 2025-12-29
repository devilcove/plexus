package agent

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	defaultWGPort         = 51820
	maxNetworks           = 100
	defaultKeepalive      = time.Second * 20
	NatsTimeout           = time.Second * 5
	NatsLongTimeout       = time.Second * 15
	checkinTime           = time.Minute * 1
	serverCheckTime       = time.Minute * 3
	connectivityTimeout   = time.Minute * 3
	endpointServerTimeout = time.Second * 30
	networkNotMapped      = "network not mapped to server"
	version               = "v0.3.0"
	networkTable          = "networks"
	deviceTable           = "devices"
)

var (
	Config        Configuration
	serverConn    atomic.Pointer[nats.Conn]
	subscriptions []*nats.Subscription
	// errors.
	ErrNetNotMapped = errors.New("network not mapped to server")
	ErrNotConnected = errors.New("not connected to server")
)

type Configuration struct {
	NatsPort  int
	Verbosity string
	DataDir   string
}
