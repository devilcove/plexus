package cmd

import (
	"net"
	"testing"

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
}

func TestStunn(t *testing.T) {
	addr := stunn()
	t.Log(addr)

}
