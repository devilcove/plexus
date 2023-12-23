package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var (
	router *gin.Engine
)

func TestMain(m *testing.M) {
	_ = boltdb.Initialize("./test.db", []string{"users", "keys", "networks", "peers", "settings", "keypairs"})
	setLogging("DEBUG")
	defer boltdb.Close()
	//checkDefaultUser()
	router = setupRouter()
	os.Exit(m.Run())
}

func TestDisplayMainPage(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	assert.Nil(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	body, err := io.ReadAll(w.Body)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, string(body), "<title>Plexus</title>")
}

func TestGetPage(t *testing.T) {
	// no user
	page := getPage("someone")
	assert.Equal(t, "v0.1.0", page.Version)
}

func TestSetPage(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		setPage(nil, "something")
		page := getPage("someuser")
		assert.Equal(t, "peers", page.Page)
	})
	t.Run("sameuser", func(t *testing.T) {
		setPage("newuser", "something")
		page := getPage("newuser")
		assert.Equal(t, "something", page.Page)
	})
}

func TestSetTheme(t *testing.T) {
	setTheme("themeuser", "black")
	page := getPage("themeuser")
	assert.Equal(t, "black", page.Theme)
}

func TestSetFont(t *testing.T) {
	setFont("fontuser", "Lato")
	page := getPage("fontuser")
	assert.Equal(t, "Lato", page.Font)
}

func TestSetRefresh(t *testing.T) {
	setRefresh("refreshuser", 2)
	page := getPage("refreshuser")
	assert.Equal(t, 2, page.Refresh)
}

func TestLogin(t *testing.T) {
	err := deleteAllUsers(true)
	assert.Nil(t, err)
	t.Run("nousers", func(t *testing.T) {
		user := plexus.User{
			Username: "admin",
			Password: "testing",
		}
		payload, err := json.Marshal(&user)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		assert.Nil(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "invalid user")
	})
	err = createTestUser(plexus.User{
		Username: "testing",
		Password: "testing",
	})
	assert.Nil(t, err)
	t.Run("wronguser", func(t *testing.T) {
		user := plexus.User{
			Username: "admin",
			Password: "testing",
		}
		payload, err := json.Marshal(&user)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		assert.Nil(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "invalid user")
	})
	t.Run("wrongpass", func(t *testing.T) {
		user := plexus.User{
			Username: "testing",
			Password: "testing2",
		}
		payload, err := json.Marshal(&user)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		assert.Nil(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "invalid user")
	})
	t.Run("valid", func(t *testing.T) {
		user := plexus.User{
			Username: "testing",
			Password: "testing",
		}
		payload, err := json.Marshal(&user)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		assert.Nil(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "<h1>Peers</h1>")
		assert.NotNil(t, w.Result().Cookies())
	})
}

func TestLogout(t *testing.T) {
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/logout", nil)
	assert.Nil(t, err)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []*http.Cookie{}, w.Result().Cookies())
}

func deleteAllUsers(deleteAll bool) (errs error) {
	users, err := boltdb.GetAll[plexus.User]("users")
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Username != "admin" || deleteAll == true {
			if err := boltdb.Delete[plexus.User](user.Username, "users"); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	return errs
}

func testLogin(data plexus.User) (*http.Cookie, error) {
	w := httptest.NewRecorder()
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "plexus" {
			return cookie, nil
		}
	}
	return nil, errors.New("no cookie")
}

func createTestUser(user plexus.User) error {
	user.Password, _ = hashPassword(user.Password)
	if err := boltdb.Save(&user, user.Username, "users"); err != nil {
		return err
	}
	return nil
}
