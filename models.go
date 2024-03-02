package plexus

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Action int

const (
	DeletePeer Action = iota
	AddPeer
	UpdatePeer
	AddRelay
	DeleteRelay
	DeleteNetwork
	JoinNetwork
	LeaveNetwork
	LeaveServer
	Ping
)

func (i Action) String() string {
	return [...]string{"DeletePeer", "AddPeer", "UpdatePeer", "AddRelay", "DelteRely",
		"DeleteNetwork", "JoinNetwork", "LeaveNetwork", "LeaveServer", "Ping"}[i]
}

type Settings struct {
	Theme   string `json:"theme" form:"theme"`
	Font    string `json:"font" form:"font"`
	Refresh int    `json:"refresh" form:"refresh"`
}

type ErrorMessage struct {
	Status  string
	Message string
}

func (e *ErrorMessage) Process(c *gin.Context) {
	slog.Error(e.Message, "status", e.Status)
	c.HTML(http.StatusOK, "error", e)
	c.Abort()
}

type User struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
	IsAdmin  bool
	Updated  time.Time
}

type Network struct {
	Name            string `form:"name"`
	ServerURL       string
	Net             net.IPNet
	AddressString   string `form:"addressstring"`
	ListenPort      int    //only used by agent
	Interface       string // only used by agent
	InterfaceSuffix int    // only used by agent
	Connected       bool
	Peers           []NetworkPeer
}

type NetworkPeer struct {
	WGPublicKey      string
	HostName         string
	Address          net.IPNet
	PublicListenPort int
	Endpoint         string
	NatsConnected    bool
	Connectivity     float64
	IsRelay          bool
	RelayedPeers     []string
	IsRelayed        bool
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
	WGPublicKey      string
	PubNkey          string
	Version          string
	Name             string
	OS               string
	ListenPort       int
	PublicListenPort int
	Endpoint         string
	Updated          time.Time
	NatsConnected    bool
}

type Device struct {
	Peer
	WGPrivateKey string
	Seed         string
	Servers      []string
}

type ServerClients struct {
	Name    string
	PubNKey string
}

type ServerRegisterRequest struct {
	KeyName string
	Peer
}

type JoinRequest struct {
	Network string
	Peer
}

type LevelRequest struct {
	Level string
}

type LeaveResponse struct {
	Error   bool
	Message string
}
type NetworkResponse struct {
	Message  string
	Error    bool
	Networks []Network
}

type LeaveRequest struct {
	Network string
}

type UpdateRequest struct {
	Network string
	Server  string
	Peer    Peer
	Action  Action
}

type NetworkUpdate struct {
	Action Action
	Peer   NetworkPeer
}

type DeviceUpdate struct {
	Action  Action
	Network Network
}

//type Command struct {
//Command string
//Data    any
//}

type RegisterRequest struct {
	Token string
}

type ConnectivityData struct {
	Network      string
	Connectivity float64
}

type CheckinData struct {
	ID               string
	Version          string
	ListenPort       int
	PublicListenPort int
	Endpoint         string
	Connections      []ConnectivityData
}

type StatusResponse struct {
	Servers  []ServerConnection
	Networks []Network
}

type ServerConnection struct {
	Server    string
	Connected string
}

type PingResponse struct {
	Message string
}

type ReloadRequest struct {
	Server string
}

type ResetRequest struct {
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
