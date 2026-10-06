package cmd

import (
	"testing"
	"time"

	"github.com/Kairum-Labs/should"
)

func TestPrintHandshake(t *testing.T) {
	t.Setenv("NO_COLOR", "true")
	//bytes := make([]byte, 128)
	//out, err := os.Open(os.Stdout.Name())
	//should.NotBeError(t, err)

	t.Run("one second", func(t *testing.T) {
		s := handshakeTime(time.Now().Add(time.Second * -1))
		should.ContainSubstring(t, s, "1 second ago")
	})
	t.Run("one minute", func(t *testing.T) {
		s := handshakeTime(time.Now().Add(time.Second * -60))
		should.ContainSubstring(t, s, "1 minute 0 seconds ago")
	})
	t.Run("hours", func(t *testing.T) {
		s := handshakeTime(time.Now().Add(time.Second * -3600))
		should.ContainSubstring(t, s, "1 hour 0 minutes 0 seconds ago")
	})
	t.Run("multi", func(t *testing.T) {
		s := handshakeTime(time.Now().Add(time.Second * -7250))
		should.ContainSubstring(t, s, "2 hours 0 minutes 50 seconds ago")
	})
	t.Run("now", func(t *testing.T) {
		s := handshakeTime(time.Now())
		should.ContainSubstring(t, s, "never")
	})
}

func TestPrettyByteSize(t *testing.T) {
	should.BeEqual(t, prettyByteSize(0), "0 B")
	should.BeEqual(t, prettyByteSize(86), "86 B")
	should.BeEqual(t, prettyByteSize(120), "120 B")
	should.BeEqual(t, prettyByteSize(1050), "1.03 KiB")
	should.BeEqual(t, prettyByteSize(9223372036854775807), "8.00 EiB")
}
