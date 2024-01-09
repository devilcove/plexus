package plexus

import (
	"errors"
	"os"
	"os/user"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAlive(t *testing.T) {
	pid := os.Getpid()
	t.Run("alive", func(t *testing.T) {
		assert.True(t, IsAlive(pid))

	})
	t.Run("dead", func(t *testing.T) {
		assert.False(t, IsAlive(pid+10))
	})
}

func TestReadPID(t *testing.T) {
	file := "/tmp/test.pid"
	t.Run("nofile", func(t *testing.T) {
		pid, err := ReadPID(file)
		assert.True(t, errors.Is(err, os.ErrNotExist))
		assert.Equal(t, 0, pid)
	})
	t.Run("invalidEntry", func(t *testing.T) {
		err := os.WriteFile(file, []byte("hello"), 0644)
		assert.Nil(t, err)
		pid, err := ReadPID(file)
		assert.True(t, errors.Is(err, strconv.ErrSyntax))
		assert.Equal(t, 0, pid)
	})
	t.Run("negativeEntry", func(t *testing.T) {
		err := os.WriteFile(file, []byte(strconv.Itoa(-1)), 0644)
		assert.Nil(t, err)
		pid, err := ReadPID(file)
		assert.Equal(t, ErrInvalidPID, err)
		assert.Equal(t, -1, pid)
	})
	t.Run("valid", func(t *testing.T) {
		err := os.WriteFile(file, []byte(strconv.Itoa(os.Getpid())), 0644)
		assert.Nil(t, err)
		pid, err := ReadPID(file)
		assert.Nil(t, err)
		assert.Equal(t, os.Getpid(), pid)
	})
	assert.Nil(t, os.Remove(file))
}

func TestWritePID(t *testing.T) {
	file := "/tmp/test.pid"
	t.Run("invalidPID", func(t *testing.T) {
		err := WritePID(file, -1)
		assert.True(t, errors.Is(err, ErrInvalidPID))
	})
	t.Run("valid", func(t *testing.T) {
		assert.Nil(t, WritePID(file, os.Getpid()))
	})
	t.Run("stillRunning", func(t *testing.T) {
		assert.True(t, errors.Is(WritePID(file, os.Getpid()), ErrProcessRunning))
	})
	user, err := user.Current()
	assert.Nil(t, err)
	if user.Uid != "0" {
		t.Run("unableToRead", func(t *testing.T) {
			assert.Nil(t, os.Chmod(file, 0333))
			assert.True(t, errors.Is(WritePID(file, 100), os.ErrPermission))
		})
	}
	assert.Nil(t, os.Remove(file))

}
