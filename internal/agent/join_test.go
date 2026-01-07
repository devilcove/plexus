package agent

import (
	"errors"
	"net"
	"os"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
)

func TestCheckPort(t *testing.T) {
	add := net.UDPAddr{
		Port: 51830,
	}
	conn, err := net.ListenUDP("udp", &add)
	should.NotBeError(t, err)
	should.NotBeNil(t, conn)
	t.Run("portnotavail", func(t *testing.T) {
		newPort := checkPort(51830)
		should.BeEqual(t, newPort, 51831)
	})
	err = conn.Close()
	should.NotBeError(t, err)
	t.Run("portavail", func(t *testing.T) {
		newPort := checkPort(51830)
		should.BeEqual(t, newPort, 51830)
	})
	t.Run("noports", func(t *testing.T) {
		add1 := net.UDPAddr{
			Port: 65534,
		}
		conn1, err := net.ListenUDP("udp", &add1)
		should.NotBeError(t, err)
		add2 := net.UDPAddr{
			Port: 65535,
		}
		conn2, err := net.ListenUDP("udp", &add2)
		should.NotBeError(t, err)
		newPort := checkPort(65534)
		should.BeEqual(t, newPort, 0)
		err = conn1.Close()
		should.NotBeError(t, err)
		err = conn2.Close()
		should.NotBeError(t, err)
	})
}

func TestNewDevice(t *testing.T) {
	// err := boltdb.Initialize("./test.db", []string{deviceTable})
	// should.NotBeError(t, err)
	device := Device{}
	err := boltdb.Delete[Device]("self", deviceTable)
	if err != nil && !errors.Is(err, boltdb.ErrNoResults) {
		t.Fail()
	}
	hostname, err := os.Hostname()
	should.NotBeError(t, err)
	t.Run("newDevice", func(t *testing.T) {
		device, err = newDevice()
		should.NotBeError(t, err)
		should.BeEqual(t, device.Name, hostname)
	})
	t.Run("existingDevice", func(t *testing.T) {
		device, err = newDevice()
		should.NotBeError(t, err)
		new, err := newDevice()
		should.NotBeError(t, err)
		should.BeEqual(t, new.Seed, device.Seed)
	})
}
