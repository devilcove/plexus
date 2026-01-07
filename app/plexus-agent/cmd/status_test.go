package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/Kairum-Labs/should"
)

func TestPrintHandshake(t *testing.T) {
	bytes := make([]byte, 128)
	out, err := os.Open(os.Stdout.Name())
	should.NotBeError(t, err)
	t.Run("one second", func(t *testing.T) {
		printHandshake(time.Now().Add(time.Second * -1))
		_, err := out.Read(bytes)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(bytes), "1 second ago")
		should.BeNil(t, out.Close())
	})
	t.Run("one minute", func(t *testing.T) {
		out, err := os.Open(os.Stdout.Name())
		should.NotBeError(t, err)
		printHandshake(time.Now().Add(time.Second * -60))
		_, err = out.Read(bytes)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(bytes), "1 minute 0 seconds ago")
		should.BeNil(t, out.Close())
	})
	t.Run("hours", func(t *testing.T) {
		out, err := os.Open(os.Stdout.Name())
		should.NotBeError(t, err)
		printHandshake(time.Now().Add(time.Second * -3600))
		_, err = out.Read(bytes)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(bytes), "1 hour 0 minutes 0 seconds ago")
		should.BeNil(t, out.Close())
	})
	t.Run("multi", func(t *testing.T) {
		out, err := os.Open(os.Stdout.Name())
		should.NotBeError(t, err)
		printHandshake(time.Now().Add(time.Second * -7250))
		_, err = out.Read(bytes)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(bytes), "2 hours 0 minutes 50 seconds ago")
		should.BeNil(t, out.Close())
	})
	t.Run("now", func(t *testing.T) {
		out, err := os.Open(os.Stdout.Name())
		should.NotBeError(t, err)
		printHandshake(time.Now())
		_, err = out.Read(bytes)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(bytes), "never")
		should.BeNil(t, out.Close())
	})
}

func TestPrettyByteSize(t *testing.T) {
	should.BeEqual(t, prettyByteSize(0), "0 B")
	should.BeEqual(t, prettyByteSize(86), "86 B")
	should.BeEqual(t, prettyByteSize(120), "120 B")
	should.BeEqual(t, prettyByteSize(1050), "1.03 KiB")
	should.BeEqual(t, prettyByteSize(9223372036854775807), "8.00 EiB")
}
