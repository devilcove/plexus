package cmd

import (
	"net"
	"os"
	"testing"

	"github.com/devilcove/plexus"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
)

func TestCheckPort(t *testing.T) {
	add := net.UDPAddr{
		Port: 51820,
	}
	conn, err := net.ListenUDP("udp", &add)
	assert.Nil(t, err)
	t.Run("portnotavail", func(t *testing.T) {
		newPort := checkPort(51820)
		assert.Equal(t, 51821, newPort)
	})
	err = conn.Close()
	assert.Nil(t, err)
	t.Run("portavail", func(t *testing.T) {
		newPort := checkPort(51820)
		assert.Equal(t, 51820, newPort)
	})
	t.Run("noports", func(t *testing.T) {
		add1 := net.UDPAddr{
			Port: 65534,
		}
		conn1, err := net.ListenUDP("udp", &add1)
		assert.Nil(t, err)
		add2 := net.UDPAddr{
			Port: 65535,
		}
		conn2, err := net.ListenUDP("udp", &add2)
		assert.Nil(t, err)
		newPort := checkPort(65534)
		assert.Equal(t, 0, newPort)
		err = conn1.Close()
		assert.Nil(t, err)
		err = conn2.Close()
		assert.Nil(t, err)
	})
}

func TestGetPubAddPort(t *testing.T) {
	addr := getPublicAddPort()
	t.Log(addr)
}

func TestSaveDevice(t *testing.T) {
	err := os.Setenv("DB_FILE", "./test.db")
	assert.Nil(t, err)
	kp, err := nkeys.CreateUser()
	assert.Nil(t, err)
	seed, err := kp.Seed()
	assert.Nil(t, err)
	peer, privKey := createPeer(string(seed))
	device := plexus.Device{
		Peer:       peer,
		PrivateKey: privKey,
		Seed:       string(seed),
		PrivKeyStr: privKey.String(),
	}
	err = saveDevice(device)
	assert.Nil(t, err)
}
