package plexus

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net"
	"time"
)

// nats topics
const (
	DeletePeer         = ".deletePeer"
	AddRouter          = ".addRouter"
	AddPeer            = ".addPeer"
	UpdatePeer         = ".updatePeer"
	UpdateNetworkPeer  = ".updateNetworkPeer"
	UpdateListenPorts  = ".updateListenPorts"
	AddRelay           = ".addRelay"
	DeleteRelay        = ".deleteRelay"
	DeleteRouter       = ".deleteRouter"
	DeleteNetwork      = ".deleteNetwork"
	JoinNetwork        = ".join"
	LeaveNetwork       = ".leaveNetwork"
	LeaveServer        = ".leaveServer"
	LogLevel           = ".loglevel"
	Ping               = ".ping"
	Register           = ".register"
	Reload             = ".reload"
	Reset              = ".reset"
	SetPrivateEndpoint = ".privateEndpoint"
	Status             = ".status"
	Version            = ".version"
	Checkin            = ".checkin"
	SendListenPorts    = ".listenPorts"
	Update             = "update."
	Networks           = "networks."
)

type ErrorResponse struct {
	Message string
}

type MessageResponse struct {
	Message string
}

type User struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
	IsAdmin  bool
	Updated  time.Time
}

type NatsUser struct {
	Kind      string
	Name      string
	Subscribe []string
	Publish   []string
}
type Network struct {
	Name          string `form:"name"`
	Net           net.IPNet
	AddressString string `form:"addressstring"`
	Peers         []NetworkPeer
}

type NetworkPeer struct {
	WGPublicKey        string
	HostName           string
	Address            net.IPNet
	ListenPort         int
	PublicListenPort   int
	Endpoint           net.IP
	PrivateEndpoint    net.IP
	UsePrivateEndpoint bool
	NatsConnected      bool
	Connectivity       float64
	IsRelay            bool
	RelayedPeers       []string
	IsRelayed          bool
	IsSubnetRouter     bool
	Subnet             net.IPNet
	UseNat             bool
	UseVirtSubnet      bool
	VirtSubnet         net.IPNet
}

type Key struct {
	Name    string `form:"name"`
	Value   string
	Usage   int `form:"usage"`
	Expires time.Time
	DispExp string `form:"expires"`
}

type KeyValue struct {
	URL     string
	Seed    string
	KeyName string
}

type Peer struct {
	WGPublicKey   string
	PubNkey       string
	Version       string
	Name          string
	OS            string
	Endpoint      net.IP
	Updated       time.Time
	NatsConnected bool
}

type ServerRegisterRequest struct {
	KeyName string
	Peer
}

type JoinRequest struct {
	Network          string
	ListenPort       int
	PublicListenPort int
	Peer
}

type ServerJoinRequest struct {
	Network Network
}

type JoinResponse struct {
	Message string
	Network Network
}

type LevelRequest struct {
	Level string
}

type LeaveRequest struct {
	Network string
}

type VersionResponse struct {
	Server string
	Agent  string
}

type NetworkUpdate struct {
	Action string
	Peer   NetworkPeer
}

type DeviceUpdate struct {
	Action  string
	Server  string
	Network Network
}

type RegisterRequest struct {
	Token string
}

type ConnectivityData struct {
	Network      string
	Connectivity float64
}

type NetworkResponse struct {
	Message  string
	Networks []Network
}

type CheckinData struct {
	ID               string
	Version          string
	ListenPort       int
	PublicListenPort int
	Endpoint         net.IP
	Connections      []ConnectivityData
}

type PingResponse struct {
	Message string
}

type ResetRequest struct {
	Network string
}

type ListenPortRequest struct {
	Network string
}

type ListenPortResponse struct {
	Message          string
	ListenPort       int
	PublicListenPort int
}

type PrivateEndpoint struct {
	IP      string
	Network string
}

func DecodeToken(token string) (KeyValue, error) {
	kv := KeyValue{}
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		slog.Error("base64 decode", "error", err)
		return kv, err
	}
	if err := json.Unmarshal(data, &kv); err != nil {
		slog.Error("token unmarshal", "error", err)
		return kv, err
	}
	return kv, nil
}
