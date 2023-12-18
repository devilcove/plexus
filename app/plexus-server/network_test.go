package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devilcove/plexus"
	"github.com/devilcove/plexus/database"
	"github.com/stretchr/testify/assert"
)

func TestDisplayAddNetwork(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(user)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	req, err := http.NewRequest(http.MethodGet, "/networks/add", nil)
	assert.Nil(t, err)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body, err := io.ReadAll(w.Body)
	assert.Nil(t, err)
	assert.Contains(t, string(body), "<h1>Add Network</h1>")
}

func TestAddNetwork(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(user)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	t.Run("emptydata", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/networks/add", nil)
		assert.Nil(t, err)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "invalid network data")
	})
	t.Run("spacesNetworkName", func(t *testing.T) {
		network := plexus.Network{
			Name:          "this has spaces",
			AddressString: "10.10.10.0/24",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "Error Processing Request: invalid network name")
	})
	t.Run("upperCase", func(t *testing.T) {
		network := plexus.Network{
			Name:          "UpperCase",
			AddressString: "10.10.10.0/24",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "Error Processing Request: invalid network name")
	})
	t.Run("nameTooLong", func(t *testing.T) {
		network := plexus.Network{
			AddressString: "10.10.10.0/24",
		}
		for i := 0; i < 300; i++ {
			network.Name = network.Name + "A"
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "Error Processing Request: invalid network name")
	})
	t.Run("invalidCIDR", func(t *testing.T) {
		network := plexus.Network{
			Name:          "cidr",
			AddressString: "10.10.10.0",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "Error Processing Request: invalid CIDR")
	})
	t.Run("normalizeCidr", func(t *testing.T) {
		network := plexus.Network{
			Name:          "normalcidr",
			AddressString: "10.10.20.100/24",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "<div id=\"error\" class=\"w3-red\"></div>")
	})

	t.Run("duplicateCidr", func(t *testing.T) {
		network := plexus.Network{
			Name:          "duplicatecidr",
			AddressString: "10.10.20.100/24",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "network CIDR in use")
	})
	t.Run("addressNotPrivate", func(t *testing.T) {
		network := plexus.Network{
			Name:          "notprivate",
			AddressString: "8.8.8.0/24",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "Error Processing Request: network address is not private")

	})
	t.Run("valid", func(t *testing.T) {
		network := plexus.Network{
			Name:          "valid",
			AddressString: "10.10.10.0/24",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "<div id=\"error\" class=\"w3-red\"></div>")
	})
	t.Run("duplicateName", func(t *testing.T) {
		network := plexus.Network{
			Name:          "valid",
			AddressString: "10.10.10.0/24",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "Error Processing Request: network name exists")
	})
	err = deleteAllNetworks()
	assert.Nil(t, err)
}

func TestDeleteNetwork(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(user)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	t.Run("nosuchnetwork", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, "/networks/network", nil)
		assert.Nil(t, err)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "network does not exist")
	})
	t.Run("existingNetwork", func(t *testing.T) {
		network := plexus.Network{
			Name:          "valid",
			AddressString: "10.10.10.0/24",
		}
		payload, err := json.Marshal(&network)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/networks/add", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.Header.Set("content-type", "application/json")
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		req, err = http.NewRequest(http.MethodDelete, "/networks/valid", nil)
		assert.Nil(t, err)
		req.AddCookie(cookie)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "<div id=\"error\" class=\"w3-red\"></div>")
	})
	err = deleteAllNetworks()
	assert.Nil(t, err)
}

func deleteAllNetworks() error {
	var errs error
	nets, err := database.GetAllNetworks()
	if err != nil {
		return err
	}
	for _, net := range nets {
		if err := database.DeleteNetwork(net.Name); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}
