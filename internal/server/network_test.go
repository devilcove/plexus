package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/plexus"
)

func TestDisplayAddNetwork(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	cookie := testLogin(t, user)
	req := httptest.NewRequest(http.MethodGet, "/networks/add", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	should.BeEqual(t, w.Code, http.StatusOK)
	body, err := io.ReadAll(w.Body)
	should.NotBeError(t, err)
	should.ContainSubstring(t, string(body), "<h1>Add Network</h1>")
}

func TestAddNetwork(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	cookie := testLogin(t, user)
	t.Run("emptydata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/networks/add", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid address")
	})
	t.Run("spacesNetworkName", func(t *testing.T) {
		payload := bodyParams("name", "this has spaces", "addressstring", "10.10.10.0/24")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid network name")
	})
	t.Run("upperCase", func(t *testing.T) {
		payload := bodyParams("name", "UpperCase", "addressstring", "10.10.10.0/24")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid network name")
	})
	t.Run("nameTooLong", func(t *testing.T) {
		var name strings.Builder
		name.WriteString("A")
		for range 300 {
			name.WriteString("A")
		}
		payload := bodyParams("name", name.String(), "addressstring", "10.10.10.0/24")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid network name")
	})
	t.Run("invalidCIDR", func(t *testing.T) {
		payload := bodyParams("name", "cidr", "addressstring", "10.10.10.0")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "invalid address for network")
	})
	t.Run("normalizeCidr", func(t *testing.T) {
		payload := bodyParams("name", "normalcidr", "addressstring", "10.10.20.100/24")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<div class=\"w3-margin-top\">10.10.20.0/24")
	})

	t.Run("duplicateCidr", func(t *testing.T) {
		payload := bodyParams("name", "duplicatecidr", "addressstring", "10.10.20.100/24")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "network CIDR in use")
	})
	t.Run("addressNotPrivate", func(t *testing.T) {
		payload := bodyParams("name", "notprivate", "addressstring", "8.8.8.0/24")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "network address is not private")
	})
	t.Run("valid", func(t *testing.T) {
		payload := bodyParams("name", "valid", "addressstring", "10.10.10.0/24")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<div class=\"w3-margin-top\">10.10.10.0/24")
	})
	t.Run("duplicateName", func(t *testing.T) {
		payload := bodyParams("name", "valid", "addressstring", "10.10.10.0/24")
		req := httptest.NewRequest(http.MethodPost, "/networks/add", payload)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "network name exists")
	})
	deleteAllNetworks(t)
}

func TestDisplayNetworkDetails(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	createTestNetwork(t)
	createTestNetworkPeer(t)

	r := httptest.NewRequest(http.MethodGet, "/networks/details/valid", nil)
	r.AddCookie(testLogin(t, user))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
	body, err := io.ReadAll(w.Body)
	should.NotBeError(t, err)
	should.ContainSubstring(t, string(body), "Network: valid")
	deleteAllNetworks(t)
}

func TestDeleteNetwork(t *testing.T) {
	setup(t)
	defer shutdown(t)
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	cookie := testLogin(t, user)
	t.Run("nosuchnetwork", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/networks/network", nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "network does not exist")
	})
	t.Run("existingNetwork", func(t *testing.T) {
		createTestNetwork(t)
		req := httptest.NewRequest(http.MethodDelete, "/networks/valid", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Networks")
	})
	deleteAllNetworks(t)
}

func TestNetworkSideBar(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	r := httptest.NewRequest(http.MethodGet, "/sidebar/", nil)
	r.AddCookie(testLogin(t, user))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
	body, err := io.ReadAll(w.Result().Body)
	should.NotBeError(t, err)
	should.ContainSubstring(t, string(body), "Networks")
}
