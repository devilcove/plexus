package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/plexus"
)

func TestRelay(t *testing.T) {
	setup(t)
	defer shutdown(t)
	deleteAllNetworks(t)
	deleteAllPeers(t)
	deleteAllUsers(t)

	user := plexus.User{Username: "admin", Password: "pass"}
	createTestUser(t, user)
	createTestNetwork(t)
	peer := createTestNetworkPeer(t)
	peer2 := createTestNetworkPeer(t)

	t.Run("displayAdd", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/networks/relay/valid/"+peer, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Create Relay")
	})

	t.Run("addInvalidNet", func(t *testing.T) {
		payload := bodyParams("relayed", peer2)
		r := httptest.NewRequest(http.MethodPost, "/networks/relay/invalid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "no results found")
	})

	t.Run("addRelay", func(t *testing.T) {
		payload := bodyParams("relayed", peer2)
		r := httptest.NewRequest(http.MethodPost, "/networks/relay/valid/"+peer, payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Network:")
	})

	t.Run("deleteInvalidNet", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/networks/relay/invalid/"+peer2, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "no results found")
	})

	t.Run("delete", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/networks/relay/valid/"+peer2, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Network: valid")
	})
}
