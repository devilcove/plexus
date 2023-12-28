package cmd

import (
	"net"
	"os"
	"testing"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
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

func TestNewDevice(t *testing.T) {
	err := boltdb.Initialize("./test.db", []string{"devices"})
	assert.Nil(t, err)
	device := plexus.Device{}
	err = boltdb.Delete[plexus.Device]("self", "devices")
	assert.Nil(t, err)
	hostname, err := os.Hostname()
	assert.Nil(t, err)
	t.Run("newDevice", func(t *testing.T) {
		device = newDevice()
		assert.Equal(t, hostname, device.Name)
	})
	t.Run("existingDevice", func(t *testing.T) {
		newDevice := newDevice()
		assert.Equal(t, device.Seed, newDevice.Seed)
	})
	err = boltdb.Close()
	assert.Nil(t, err)

}
