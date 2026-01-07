package server

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	createTestUser(t, user)
	r := httptest.NewRequest(http.MethodGet, "/keys/", nil)
	r.AddCookie(testLogin(t, user))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	should.BeEqual(t, w.Code, http.StatusOK)
	body, err := io.ReadAll(w.Body)
	should.NotBeError(t, err)
	should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
}

func TestDisplayAddKey(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	cookie := testLogin(t, user)

	req := httptest.NewRequest(http.MethodGet, "/keys/add", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	should.BeEqual(t, w.Code, http.StatusOK)
	body, err := io.ReadAll(w.Body)
	should.NotBeError(t, err)
	should.ContainSubstring(t, string(body), "<h1>Create Key</h1>")
}

func TestAddKey(t *testing.T) {
	deleteAllKeys(t)
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	cookie := testLogin(t, user)
	setup(t)
	defer shutdown(t)

	t.Run("emptydata", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/keys/add", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid key")
	})
	t.Run("spaceInName", func(t *testing.T) {
		payload := bodyParams("name", "this has spaces", "expires", time.Now().Format("2006-01-02"))
		r := httptest.NewRequest(http.MethodPost, "/keys/add", payload)
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid chars")
	})
	t.Run("uppercase", func(t *testing.T) {
		payload := bodyParams("name", "Uppercase", "expires", time.Now().Format("2006-01-02"))
		r := httptest.NewRequest(http.MethodPost, "/keys/add", payload)
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid chars")
	})
	t.Run("nameTooLong", func(t *testing.T) {
		var tmp strings.Builder
		for range 256 {
			tmp.WriteString("A")
		}
		name := tmp.String()
		payload := bodyParams("name", name, "expires", time.Now().Format("2006-01-02"))
		r := httptest.NewRequest(http.MethodPost, "/keys/add", payload)
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "too long")
	})
	t.Run("invalidDate", func(t *testing.T) {
		payload := bodyParams("name", "key", "expires", time.Now().Format("2006-01-02 03-04"))
		r := httptest.NewRequest(http.MethodPost, "/keys/add", payload)
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "parsing time")
	})
	t.Run("zeroDate", func(t *testing.T) {
		payload := bodyParams("name", "key", "expires", time.Time{}.Format("2006-01-02"))
		r := httptest.NewRequest(http.MethodPost, "/keys/add", payload)
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
		keys, err := boltdb.GetAll[plexus.Key](keyTable)
		should.NotBeError(t, err)
		should.BeEqual(
			t,
			keys[0].Expires.Format("2006-01-02 03-04"),
			time.Now().Add(24*time.Hour).Format("2006-01-02 03-04"),
		)
	})
	t.Run("valid", func(t *testing.T) {
		payload := bodyParams("name", "valid", "expires",
			time.Now().Format("2006-01-02"), "usage", "0")
		r := httptest.NewRequest(http.MethodPost, "/keys/add", payload)
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
	})
	t.Run("duplicate", func(t *testing.T) {
		payload := bodyParams("name", "valid", "expires", time.Now().Format("2006-01-02"))
		r := httptest.NewRequest(http.MethodPost, "/keys/add", payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "key exists")
	})
	deleteAllKeys(t)
}

func TestDeleteKeys(t *testing.T) {
	setup(t)
	defer shutdown(t)

	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	cookie := testLogin(t, user)

	t.Run("nosuchkey", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/keys/network", nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "key does not exist")
	})
	t.Run("existingKey", func(t *testing.T) {
		// create key
		payload := bodyParams("name", "valid", "expires",
			time.Now().Format("2006-01-02"), "usage", "0")
		r := httptest.NewRequest(http.MethodPost, "/keys/add", payload)
		r.AddCookie(cookie)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
		// delete key
		r = httptest.NewRequest(http.MethodDelete, "/keys/valid", nil)
		r.AddCookie(cookie)
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err = io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Plexus Keys</h1>")
	})
	deleteAllKeys(t)
}

func TestUpdateKey(t *testing.T) {
	setup(t)
	defer shutdown(t)
	value, err := newValue("one")
	should.NotBeError(t, err)
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
	should.NotBeError(t, err)
	err = boltdb.Save(key2, key2.Name, keyTable)
	should.NotBeError(t, err)
	t.Run("keyDoesNotExist", func(t *testing.T) {
		err := decrementKeyUsage("doesnotexist")
		should.NotBeNil(t, err)
		should.BeTrue(t, errors.Is(err, boltdb.ErrNoResults))
	})
	t.Run("deleteKey", func(t *testing.T) {
		err := decrementKeyUsage(key1.Name)
		should.NotBeError(t, err)
		newKey, err := boltdb.Get[plexus.Key](key1.Name, keyTable)
		should.BeEqual(t, newKey, plexus.Key{})
		should.BeTrue(t, errors.Is(err, boltdb.ErrNoResults))
	})
	t.Run("decrement usage", func(t *testing.T) {
		err := decrementKeyUsage(key2.Name)
		should.NotBeError(t, err)
	})
	deleteAllKeys(t)
}

func TestExpireKeys(t *testing.T) {
	setup(t)
	defer shutdown(t)
	key := plexus.Key{
		Name:    "testkey",
		Expires: time.Now().Add(-1 * time.Hour),
	}
	err := boltdb.Save(key, key.Name, keyTable)
	should.NotBeError(t, err)
	keys, err := boltdb.GetAll[plexus.Key](keyTable)
	should.NotBeError(t, err)
	should.BeEqual(t, len(keys), 1)
	expireKeys()
	keys, err = boltdb.GetAll[plexus.Key](keyTable)
	should.NotBeError(t, err)
	should.BeEqual(t, len(keys), 0)
}
