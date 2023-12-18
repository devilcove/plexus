package database

import (
	"errors"
	"testing"

	"github.com/devilcove/plexus"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSaveKey(t *testing.T) {
	key := plexus.Key{
		Name:  "net1",
		ID:    uuid.New().String(),
		Usage: 1,
	}
	err := SaveKey(&key)
	assert.Nil(t, err)
	err = deleteAllKeys()
	assert.Nil(t, err)
}

func TestGetKey(t *testing.T) {
	key, err := createKey()
	assert.Nil(t, err)
	t.Run("wrongName", func(t *testing.T) {
		net, err := GetKey("net2")
		assert.NotNil(t, err)
		assert.Equal(t, "no results found", err.Error())
		assert.Equal(t, plexus.Key{}, net)
	})
	t.Run("success", func(t *testing.T) {
		net, err := GetKey(key)
		assert.Nil(t, err)
		assert.Equal(t, "net1", net.Name)
	})
	err = deleteAllKeys()
	assert.Nil(t, err)
}

func TestGetAllKeys(t *testing.T) {
	_, err := createKey()
	assert.Nil(t, err)
	t.Run("success", func(t *testing.T) {
		nets, err := GetAllKeys()
		assert.Nil(t, err)
		assert.Equal(t, 1, len(nets))
	})
	t.Run("nonets", func(t *testing.T) {
		err := deleteAllKeys()
		assert.Nil(t, err)
		nets, err := GetAllKeys()
		assert.Nil(t, err)
		assert.Equal(t, 0, len(nets))
	})
	err = deleteAllKeys()
	assert.Nil(t, err)
}

func TestDeleteKey(t *testing.T) {
	key, err := createKey()
	assert.Nil(t, err)
	t.Run("wrongID", func(t *testing.T) {
		err := DeleteKey(uuid.New().String())
		assert.NotNil(t, err)
		assert.Equal(t, "no results found", err.Error())
	})
	t.Run("valid", func(t *testing.T) {
		err := DeleteKey(key)
		assert.Nil(t, err)
	})
	t.Run("nilKey", func(t *testing.T) {
		err := DeleteKey(key)
		assert.NotNil(t, err)
		assert.Equal(t, "no results found", err.Error())
	})
	err = deleteAllKeys()
	assert.Nil(t, err)
}

func createKey() (string, error) {
	key := plexus.Key{
		Name:  "net1",
		ID:    uuid.New().String(),
		Usage: 1,
	}
	if err := SaveKey(&key); err != nil {
		return "", err
	}
	return key.ID, nil
}

func deleteAllKeys() (errs error) {
	keys, err := GetAllKeys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := DeleteKey(key.ID); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}
