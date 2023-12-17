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

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/database"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var (
	router *gin.Engine
)

func TestMain(m *testing.M) {
	os.Setenv("DB_FILE", "test.db")
	_ = database.InitializeDatabase()
	setLogging("DEBUG")
	defer database.Close()
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

func TestSetTheme(t *testing.T) {
	SetTheme("themeuser", "black")
	page := getPage("themeuser")
	assert.Equal(t, "black", page.Theme)
}

func TestSetFont(t *testing.T) {
	SetFont("fontuser", "Lato")
	page := getPage("fontuser")
	assert.Equal(t, "Lato", page.Font)
}

func TestSetRefresh(t *testing.T) {
	SetRefresh("refreshuser", 2)
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
	users, err := database.GetAllUsers()
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Username != "admin" || deleteAll == true {
			if err := database.DeleteUser(user.Username); err != nil {
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
	user.Password, _ = database.HashPassword(user.Password)
	if err := database.SaveUser(&user); err != nil {
		return err
	}
	return nil
}
