package agent

import (
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nkeys"
	"github.com/pion/stun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func processRegistration(in []byte) []byte {
	request := &plexus.RegisterRequest{}
	if err := json.Unmarshal(in, request); err != nil {
		slog.Error("invalid registration request", "error", err, "data", string(in))
		return []byte{}
	}
	response := handleRegistration(request)
	bytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("invalid registartion response", "error", err, "data", response)
	}
	return bytes

}
func handleRegistration(request *plexus.RegisterRequest) plexus.MessageResponse {
	self, err := newDevice()
	if err != nil {
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	if self.Server != "" {
		return plexus.MessageResponse{Message: "already registered with server " + self.Server}
	}
	log.Println("register request")
	loginKey, err := plexus.DecodeToken(request.Token)
	if err != nil {
		log.Println(err)
		return plexus.MessageResponse{Message: "invalid registration key: " + err.Error()}
	}
	conn, err := createRegistationConnection(loginKey)
	if err != nil {
		return plexus.MessageResponse{Message: "invalid registration key: " + err.Error()}
	}
	resp := plexus.MessageResponse{}
	serverRequest := plexus.ServerRegisterRequest{
		KeyName: loginKey.KeyName,
		Peer:    self.Peer,
	}
	if err := Request(conn, "register", serverRequest, &resp, NatsTimeout); err != nil {
		log.Println(err)
		return plexus.MessageResponse{Message: "error: " + err.Error()}
	}
	self.Server = conn.ConnectedUrl()
	if err := boltdb.Save(self, "self", deviceTable); err != nil {
		slog.Error("save device", "error", err)
		return plexus.MessageResponse{Message: "error saving device " + err.Error()}
	}
	slog.Debug("server response to join request", "response", resp)
	if err := connectToServer(self); err != nil {
		slog.Error("connect to server", "error", err)
	}
	return resp
}

func newDevice() (Device, error) {
	device, err := boltdb.Get[Device]("self", deviceTable)
	if err == nil {
		if device.Version != version {
			device.Version = version
			if err := boltdb.Save(device, "self", deviceTable); err != nil {
				slog.Error("update self version", "error", err)
			}
		}
		return device, nil
	}
	if !errors.Is(err, boltdb.ErrNoResults) {
		return device, err
	}
	peer, privKey, seed, err := createPeer()
	if err != nil {
		return device, err
	}
	device = Device{
		Peer:         *peer,
		Seed:         seed,
		WGPrivateKey: privKey.String(),
	}
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		slog.Error("unable to mkdir", "path", path, "error", err)
		return device, err
	}
	if err := os.WriteFile(path+"agent.seed", []byte(seed), os.ModePerm); err != nil {
		slog.Error("save seed", "error", err)
	}
	err = boltdb.Save(device, "self", deviceTable)
	return device, err
}

func createPeer() (*plexus.Peer, *wgtypes.Key, string, error) {
	empty := ""
	kp, err := nkeys.CreateUser()
	if err != nil {
		return nil, nil, empty, err
	}
	seed, err := kp.Seed()
	if err != nil {
		return nil, nil, empty, err
	}
	nkey, err := kp.PublicKey()
	if err != nil {
		return nil, nil, empty, err
	}
	name, err := os.Hostname()
	if err != nil {
		return nil, nil, empty, err
	}
	privKey, pubKey, err := generateKeys()
	if err != nil {
		return nil, nil, empty, err
	}
	if strings.Contains(pubKey.String(), "/") {
		return nil, nil, empty, errors.New("invalid public key")
	}
	port := checkPort(51820)
	stunAddr, err := getPublicAddPort(port)
	if err != nil {
		return nil, nil, empty, err
	}
	peer := &plexus.Peer{
		WGPublicKey: pubKey.String(),
		PubNkey:     nkey,
		Name:        name,
		Version:     version,
		Endpoint:    stunAddr.IP,
		OS:          runtime.GOOS,
		Updated:     time.Now(),
	}
	return peer, privKey, string(seed), nil
}

// generateKeys generates wgkeys that do not have a / in pubkey
func generateKeys() (*wgtypes.Key, *wgtypes.Key, error) {
	for {
		priv, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, nil, err
		}
		pub := priv.PublicKey()
		if !strings.Contains(pub.String(), "/") {
			return &priv, &pub, nil
		}
	}
}

func checkPort(rangestart int) int {
	addr := net.UDPAddr{}
	for x := rangestart; x <= 65535; x++ {
		addr.Port = x
		conn, err := net.ListenUDP("udp", &addr)
		if err != nil {
			continue
		}
		conn.Close()
		return x
	}
	return 0
}

func getPublicAddPort(port int) (*stun.XORMappedAddress, error) {
	add := &stun.XORMappedAddress{}
	stunServer, err := net.ResolveUDPAddr("udp4", "stun1.l.google.com:19302")
	if err != nil {
		return nil, err
	}
	local := &net.UDPAddr{
		IP:   net.ParseIP(""),
		Port: port,
	}
	c, err := net.DialUDP("udp4", local, stunServer)
	if err != nil {
		return nil, err
	}
	conn, err := stun.NewClient(c)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	msg := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	if err := conn.Do(msg, func(res stun.Event) {
		if err := add.GetFrom(res.Message); err != nil {
			slog.Error("get address", "error", err)
		}
	}); err != nil {
		return nil, err
	}
	return add, nil
}
