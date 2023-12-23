package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/stretchr/testify/assert"
)

func TestDisplayKeys(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(user)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	req, err := http.NewRequest(http.MethodGet, "/keys/", nil)
	assert.Nil(t, err)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body, err := io.ReadAll(w.Body)
	assert.Nil(t, err)
	assert.Contains(t, string(body), "<h1>Plexus Keys</h1>")
}

func TestDisplayAddKey(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(user)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	req, err := http.NewRequest(http.MethodGet, "/keys/add", nil)
	assert.Nil(t, err)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body, err := io.ReadAll(w.Body)
	assert.Nil(t, err)
	assert.Contains(t, string(body), "<h1>Create Key</h1>")
}

func TestAddKey(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(user)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	t.Run("emptydata", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/keys/add", nil)
		assert.Nil(t, err)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "invalid key data")
	})
	t.Run("spaceInName", func(t *testing.T) {
		key := plexus.Key{
			Name:    "this has spaces",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "invalid chars")
	})
	t.Run("uppercase", func(t *testing.T) {
		key := plexus.Key{
			Name:    "Uppercase",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "invalid chars")
	})
	t.Run("nameTooLong", func(t *testing.T) {
		key := plexus.Key{
			DispExp: time.Now().Format("2006-01-02"),
		}
		for i := 0; i < 256; i++ {
			key.Name = key.Name + "A"
		}
		payload, err := json.Marshal(&key)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "too long")
	})
	t.Run("invalidDate", func(t *testing.T) {
		key := plexus.Key{
			Name:    "Uppercase",
			DispExp: time.Now().Format("2006-01-02 03-04"),
		}
		payload, err := json.Marshal(&key)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "parsing time")
	})
	t.Run("zeroDate", func(t *testing.T) {
		key := plexus.Key{
			Name:    "valid",
			DispExp: time.Time{}.Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "<h1>Plexus Keys</h1>")
		keys, err := boltdb.GetAll[plexus.Key]("keys")
		assert.Nil(t, err)
		assert.Equal(t, time.Now().Add(24*time.Hour).Format("2006-01-02 03-04"), keys[0].Expires.Format("2006-01-02 03-04"))
	})
	t.Run("valid", func(t *testing.T) {
		key := plexus.Key{
			Name:    "valid",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "<h1>Plexus Keys</h1>")
	})
	err = deleteAllKeys()
	assert.Nil(t, err)
	t.Log(err)
}

func TestDeleteKeys(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(user)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	t.Run("nosuchkey", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, "/keys/network", nil)
		assert.Nil(t, err)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "key does not exist")
	})
	t.Run("existingKey", func(t *testing.T) {
		key := plexus.Key{
			Name:    "valid",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "<h1>Plexus Keys</h1>")
		req, err = http.NewRequest(http.MethodDelete, "/keys/valid", nil)
		assert.Nil(t, err)
		req.AddCookie(cookie)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err = io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "<h1>Plexus Keys</h1>")
	})
	err = deleteAllKeys()
	assert.Nil(t, err)
}

func deleteAllKeys() error {
	var errs error
	keys, err := boltdb.GetAll[plexus.Key]("keys")
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := boltdb.Delete[plexus.Key](key.Name, "keys"); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}
