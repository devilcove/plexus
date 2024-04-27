package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPrintHandshake(t *testing.T) {
	bytes := make([]byte, 128)
	out, err := os.Open(os.Stdout.Name())
	assert.Nil(t, err)
	t.Run("one second", func(t *testing.T) {
		printHandshake(time.Now().Add(time.Second * -1))
		_, err := out.Read(bytes)
		assert.Nil(t, err)
		assert.Contains(t, string(bytes), "1 second ago")
		assert.Nil(t, out.Close())
	})
	t.Run("one minute", func(t *testing.T) {
		out, err := os.Open(os.Stdout.Name())
		assert.Nil(t, err)
		printHandshake(time.Now().Add(time.Second * -60))
		_, err = out.Read(bytes)
		assert.Nil(t, err)
		assert.Contains(t, string(bytes), "1 minute 0 seconds ago")
		assert.Nil(t, out.Close())
	})
	t.Run("hours", func(t *testing.T) {
		out, err := os.Open(os.Stdout.Name())
		assert.Nil(t, err)
		printHandshake(time.Now().Add(time.Second * -3600))
		_, err = out.Read(bytes)
		assert.Nil(t, err)
		assert.Contains(t, string(bytes), "1 hour 0 minutes 0 seconds ago")
		assert.Nil(t, out.Close())
	})
	t.Run("multi", func(t *testing.T) {
		out, err := os.Open(os.Stdout.Name())
		assert.Nil(t, err)
		printHandshake(time.Now().Add(time.Second * -7250))
		_, err = out.Read(bytes)
		assert.Nil(t, err)
		assert.Contains(t, string(bytes), "2 hours 0 minutes 50 seconds ago")
		assert.Nil(t, out.Close())
	})
	t.Run("now", func(t *testing.T) {
		out, err := os.Open(os.Stdout.Name())
		assert.Nil(t, err)
		printHandshake(time.Now())
		_, err = out.Read(bytes)
		assert.Nil(t, err)
		assert.Contains(t, string(bytes), "never")
		assert.Nil(t, out.Close())
	})
}

func TestPrettyByteSize(t *testing.T) {
	assert.Equal(t, "0 B", prettyByteSize(0))
	assert.Equal(t, "86 B", prettyByteSize(86))
	assert.Equal(t, "120 B", prettyByteSize(120))
	assert.Equal(t, "1.03 KiB", prettyByteSize(1050))
	assert.Equal(t, "8.00 EiB", prettyByteSize(9223372036854775807))
}
