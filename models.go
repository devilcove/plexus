package plexus

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/gin-gonic/gin"
)

type Config struct {
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
	Name          string `form:"name"`
	Net           iplib.Net4
	Address       netip.Addr
	AddressString string `form:"addressstring"`
	Peers         []NetworkPeer
}

type NetworkPeer struct {
	WGPublicKey      string
	Address          net.IP
	PublicListenPort int
	Endpoint         string
}

type Key struct {
	Name     string `form:"name"`
	Value    string
	Usage    int `form:"usage"`
	Expires  time.Time
	DispExp  string   `form:"expires"`
	Networks []string `form:"networks"`
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
	Networks         []string
}

type Device struct {
	Peer
	WGPrivateKey string
	Seed         string
}

type ServerClients struct {
	Name    string
	PubNKey string
}

type JoinRequest struct {
	KeyName string
	Peer
}

func DecodeToken(token string) (KeyValue, error) {
	kv := KeyValue{}
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		slog.Error("base64 decode", "error", err)
		return kv, err
	}
	slog.Info(string(data))
	if err := json.Unmarshal(data, &kv); err != nil {
		slog.Error("token unmarshal", "error", err)
		return kv, err
	}
	return kv, nil
}
