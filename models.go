package plexus

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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
	Address       net.IPNet
	AddressString string `form:"addressstring"`
}

type Key struct {
	Name     string `form:"name"`
	Value    string
	Usage    int `form:"usage"`
	Expires  time.Time
	DispExp  string   `form:"expires"`
	Networks []string `form:"networks"`
}

type Peer struct {
	PublicKey        wgtypes.Key
	PubNkey          string
	Version          string
	Name             string
	OS               string
	ListenPort       int
	PublicListenPort int
	Endpoint         net.IP
	Updated          time.Time
}

type Device struct {
	Peer
	PrivateKey wgtypes.Key
	Seed       string
}

type ServerClients struct {
	Name    string
	PubNKey string
}
