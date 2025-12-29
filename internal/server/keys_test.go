package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func TestDisplayKeys(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	err := createTestUser(user)
	should.BeNil(t, err)
	cookie, err := testLogin(user)
	should.BeNil(t, err)
	should.NotBeNil(t, cookie)
	req, err := http.NewRequest(http.MethodGet, "/keys/", nil)
	should.BeNil(t, err)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	should.BeEqual(t, w.Code, http.StatusOK)
	body, err := io.ReadAll(w.Body)
	should.BeNil(t, err)
	should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
}

func TestDisplayAddKey(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	err := createTestUser(user)
	should.BeNil(t, err)
	cookie, err := testLogin(user)
	should.BeNil(t, err)
	req, err := http.NewRequest(http.MethodGet, "/keys/add", nil)
	should.BeNil(t, err)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	should.BeEqual(t, w.Code, http.StatusOK)
	body, err := io.ReadAll(w.Body)
	should.BeNil(t, err)
	should.ContainSubstring(t, string(body), "<h1>Create Key</h1>")
}

func TestAddKey(t *testing.T) {
	t.Skip()
	err := deleteAllKeys()
	should.BeNil(t, err)
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	err = createTestUser(user)
	should.BeNil(t, err)
	cookie, err := testLogin(user)
	should.BeNil(t, err)
	t.Run("emptydata", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/keys/add", nil)
		should.BeNil(t, err)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "Error Processing Request")
	})
	t.Run("spaceInName", func(t *testing.T) {
		key := plexus.Key{
			Name:    "this has spaces",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		should.BeNil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "invalid chars")
	})
	t.Run("uppercase", func(t *testing.T) {
		key := plexus.Key{
			Name:    "Uppercase",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		should.BeNil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "invalid chars")
	})
	t.Run("nameTooLong", func(t *testing.T) {
		key := plexus.Key{
			DispExp: time.Now().Format("2006-01-02"),
		}
		for range 256 {
			key.Name += "A"
		}
		payload, err := json.Marshal(&key)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		should.BeNil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "too long")
	})
	t.Run("invalidDate", func(t *testing.T) {
		key := plexus.Key{
			Name:    "Uppercase",
			DispExp: time.Now().Format("2006-01-02 03-04"),
		}
		payload, err := json.Marshal(&key)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		should.BeNil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "parsing time")
	})
	t.Run("zeroDate", func(t *testing.T) {
		key := plexus.Key{
			Name:    "zerodate",
			DispExp: time.Time{}.Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		should.BeNil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
		keys, err := boltdb.GetAll[plexus.Key](keyTable)
		should.BeNil(t, err)
		should.BeEqual(
			t,
			keys[0].Expires.Format("2006-01-02 03-04"),
			time.Now().Add(24*time.Hour).Format("2006-01-02 03-04"),
		)
	})
	t.Run("valid", func(t *testing.T) {
		key := plexus.Key{
			Name:    "valid",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		should.BeNil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
	})
	t.Run("duplicate", func(t *testing.T) {
		key := plexus.Key{
			Name:    "valid",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		should.BeNil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "key exists")
	})
	err = deleteAllKeys()
	should.BeNil(t, err)
}

func TestDeleteKeys(t *testing.T) {
	t.Skip()
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	err := createTestUser(user)
	should.BeNil(t, err)
	cookie, err := testLogin(user)
	should.BeNil(t, err)
	t.Run("nosuchkey", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, "/keys/network", nil)
		should.BeNil(t, err)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "key does not exist")
	})
	t.Run("existingKey", func(t *testing.T) {
		key := plexus.Key{
			Name:    "valid",
			DispExp: time.Now().Format("2006-01-02"),
		}
		payload, err := json.Marshal(&key)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/keys/add", bytes.NewBuffer(payload))
		should.BeNil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
		req, err = http.NewRequest(http.MethodDelete, "/keys/valid", nil)
		should.BeNil(t, err)
		req.AddCookie(cookie)
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err = io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
	})
	err = deleteAllKeys()
	should.BeNil(t, err)
}

func TestUpdateKey(t *testing.T) {
	t.Skip()
	value, err := newValue("one")
	should.BeNil(t, err)
	key1 := plexus.Key{
		Name:  "one",
		Usage: 1,
		Value: value,
	}
	key2 := plexus.Key{
		Name:  "two",
		Usage: 10,
	}
	err = boltdb.Save(key1, key1.Name, keyTable)
	should.BeNil(t, err)
	err = boltdb.Save(key2, key2.Name, keyTable)
	should.BeNil(t, err)
	t.Run("keyDoesNotExist", func(t *testing.T) {
		err := decrementKeyUsage("doesnotexist")
		should.NotBeNil(t, err)
		should.BeTrue(t, errors.Is(err, boltdb.ErrNoResults))
	})
	t.Run("deleteKey", func(t *testing.T) {
		err := decrementKeyUsage(key1.Name)
		should.BeNil(t, err)
		newKey, err := boltdb.Get[plexus.Key](key1.Name, keyTable)
		should.BeEqual(t, newKey, plexus.Key{})
		should.BeTrue(t, errors.Is(err, boltdb.ErrNoResults))
	})
	t.Run("decrement usage", func(t *testing.T) {
		err := decrementKeyUsage(key2.Name)
		should.BeNil(t, err)
	})
	err = deleteAllKeys()
	should.BeNil(t, err)
}

func deleteAllKeys() error {
	var errs error
	keys, err := boltdb.GetAll[plexus.Key](keyTable)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := boltdb.Delete[plexus.Key](key.Name, keyTable); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}
