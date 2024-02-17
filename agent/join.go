package agent

import (
	"errors"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/pion/stun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func processJoin(request *plexus.JoinCommand) error {
	loginKey, err := plexus.DecodeToken(request.Token)
	if err != nil {
		return err
	}
	loginKeyPair, err := nkeys.FromSeed([]byte(loginKey.Seed))
	if err != nil {
		return err
	}
	loginPublicKey, err := loginKeyPair.PublicKey()
	if err != nil {
		return err
	}
	sign := func(nonce []byte) ([]byte, error) {
		return loginKeyPair.Sign(nonce)
	}
	device, err := newDevice()
	if err != nil {
		return err
	}
	joinRequest := plexus.JoinRequest{
		KeyName: loginKey.KeyName,
		Peer:    device.Peer,
	}
	opts := nats.Options{
		Url:         loginKey.URL,
		Nkey:        loginPublicKey,
		SignatureCB: sign,
	}
	nc, err := opts.Connect()
	if err != nil {
		return err
	}
	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return err
	}
	resp := plexus.NetworkResponse{}
	if err := ec.Request("join", joinRequest, &resp, NatsTimeout); err != nil {
		return err
	}
	self, err := boltdb.Get[plexus.Device]("self", "devices")
	if err != nil {
		slog.Error("get self", "error", err)
	}
	addNewNetworks(self, resp.Networks)
	// reset nats connection
	connectToServers()
	return nil
}

func newDevice() (plexus.Device, error) {
	device, err := boltdb.Get[plexus.Device]("self", "devices")
	if err == nil {
		return device, nil
	}
	if !errors.Is(err, boltdb.ErrNoResults) {
		return device, err
	}
	peer, privKey, seed, err := createPeer()
	if err != nil {
		return device, err
	}
	device = plexus.Device{
		Peer:         *peer,
		Seed:         seed,
		WGPrivateKey: privKey.String(),
	}
	err = boltdb.Save(device, "self", "devices")
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
	stunAddr, err := getPublicAddPort()
	if err != nil {
		return nil, nil, empty, err
	}
	peer := &plexus.Peer{
		WGPublicKey:      pubKey.String(),
		PubNkey:          nkey,
		Name:             name,
		Version:          "v0.1.0",
		ListenPort:       port,
		PublicListenPort: stunAddr.Port,
		Endpoint:         stunAddr.IP.String(),
		OS:               runtime.GOOS,
		Updated:          time.Now(),
	}
	return peer, privKey, string(seed), nil
}

// generateKeys generates wgkeys that do not have a / in pubkey
func generateKeys() (*wgtypes.Key, *wgtypes.Key, error) {
	for {
		priv, err := wgtypes.GenerateKey()
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

func getPublicAddPort() (*stun.XORMappedAddress, error) {
	add := &stun.XORMappedAddress{}
	stunServer, err := net.ResolveUDPAddr("udp4", "stun1.l.google.com:19302")
	if err != nil {
		return nil, err
	}
	local := &net.UDPAddr{
		IP:   net.ParseIP(""),
		Port: 51820,
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
		add.GetFrom(res.Message)
	}); err != nil {
		return nil, err
	}
	return add, nil
}
