package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
)

func TestDisplayPeers(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	createTestNetwork(t)
	setup(t)
	defer shutdown(t)
	peerID := createTestNetworkPeer(t)

	t.Run("displayPeers", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/peers/", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
	})

	t.Run("peerDetails", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/peers/"+peerID, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Peer: testing")
	})

	t.Run("peerDelete", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/peers/"+peerID, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		_, err := boltdb.Get[plexus.Peer](peerID, peerTable)
		should.BeErrorIs(t, err, boltdb.ErrNoResults)
	})
}

func TestAddPeerToNetwork(t *testing.T) {
	deleteAllNetworks(t)
	deleteAllPeers(t)
	deleteAllUsers(t)
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	createTestNetwork(t)
	setup(t)
	defer shutdown(t)
	peerID := createTestPeer(t)

	t.Run("ok", func(t *testing.T) {
		t.Skip() // changes required to addPeer when not connected
		r := httptest.NewRequest(http.MethodPost, "/networks/addPeer/valid/"+peerID, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
	})

	t.Run("invalidNetwork", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/networks/addPeer/invalid/"+peerID, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusInternalServerError)
	})
	t.Run("invalidPeer", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/networks/addPeer/valid/invalid", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusInternalServerError)
	})
}

func TestRemovePeerFromNetwork(t *testing.T) {
	deleteAllNetworks(t)
	deleteAllPeers(t)
	deleteAllUsers(t)
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	createTestNetwork(t)
	setup(t)
	defer shutdown(t)
	peer := createTestNetworkPeer(t)

	t.Run("invalidPeer", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/networks/peers/valid/something", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
	})

	t.Run("valid", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/networks/peers/valid/"+peer, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
	})
}

func TestDisplayNetworkPeer(t *testing.T) {
	deleteAllNetworks(t)
	deleteAllPeers(t)
	deleteAllUsers(t)
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	createTestNetwork(t)
	setup(t)
	defer shutdown(t)
	peer := createTestNetworkPeer(t)

	t.Run("invalidNetwork", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/networks/peers/invalid/peer", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
	})

	t.Run("invalidPeer", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/networks/peers/valid/something", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
	})

	t.Run("valid", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/networks/peers/valid/"+peer, nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
	})
}

func TestGetNetworksForPeer(t *testing.T) {
	deleteAllNetworks(t)
	deleteAllPeers(t)
	deleteAllUsers(t)
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	createTestNetwork(t)
	setup(t)
	defer shutdown(t)

	t.Run("noNetwork", func(t *testing.T) {
		networks, err := getNetworksForPeer("wireguardkey")
		should.NotBeError(t, err)
		should.BeEmpty(t, networks)
	})

	t.Run("good", func(t *testing.T) {
		createTestNetwork(t)
		peer := createTestNetworkPeer(t)
		networks, err := getNetworksForPeer(peer)
		should.NotBeError(t, err)
		should.BeEqual(t, len(networks), 1)
	})
}
