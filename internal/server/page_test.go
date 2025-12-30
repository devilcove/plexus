package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/plexus"
)

func TestDisplayMainPage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	body, err := io.ReadAll(w.Body)
	should.NotBeError(t, err)
	should.BeEqual(t, w.Code, http.StatusOK)
	should.ContainSubstring(t, string(body), "<title>Plexus</title>")
}

func TestGetPage(t *testing.T) {
	// no user
	page := getPage("someone")
	should.BeEqual(t, page.Version, version)
}

func TestLogin(t *testing.T) {
	deleteAllUsers(t)
	t.Run("nousers", func(t *testing.T) {
		payload := bodyParams("username", "admin", "password", "testing")
		req := httptest.NewRequest(http.MethodPost, "/login/", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid user")
	})
	createTestUser(t, plexus.User{
		Username: "testing",
		Password: "testing",
		IsAdmin:  true,
	})
	t.Run("wronguser", func(t *testing.T) {
		payload := bodyParams("username", "admin", "password", "testing")
		req := httptest.NewRequest(http.MethodPost, "/login/", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid user")
	})
	t.Run("wrongpass", func(t *testing.T) {
		payload := bodyParams("username", "testing", "password", "testing2")
		req := httptest.NewRequest(http.MethodPost, "/login/", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid user")
	})
	t.Run("valid", func(t *testing.T) {
		payload := bodyParams("username", "testing", "password", "testing")
		req := httptest.NewRequest(http.MethodPost, "/login/", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<title>Plexus</title>")
		should.NotBeNil(t, w.Result().Cookies())
	})
}

func TestLogout(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/logout/", nil)
	router.ServeHTTP(w, req)
	should.BeEqual(t, w.Code, http.StatusFound)
	should.BeEqual(t, w.Result().Cookies()[0].MaxAge, -1)
}
