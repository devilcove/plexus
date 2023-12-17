package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devilcove/plexus"
	"github.com/stretchr/testify/assert"
)

func TestGetConfig(t *testing.T) {
	user := plexus.User{
		Username: "testing",
		Password: "password",
	}
	err := createTestUser(user)
	assert.Nil(t, err)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	assert.NotNil(t, cookie)
	req, err := http.NewRequest(http.MethodGet, "/config/", nil)
	assert.Nil(t, err)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body, err := io.ReadAll(w.Body)
	assert.Nil(t, err)
	assert.Contains(t, string(body), "<h1>Configuration</h1>")
}

func TestSetConfig(t *testing.T) {
	user := plexus.User{
		Username: "testing",
		Password: "testing",
	}
	err := createTestUser(user)
	assert.Nil(t, err)
	cookie, err := testLogin(user)
	assert.Nil(t, err)
	assert.NotNil(t, cookie)
	t.Run("noconfig", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "/config/", nil)
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("content-type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "invalid config")
	})
	t.Run("validConfig", func(t *testing.T) {
		config := plexus.Config{
			Theme: "red",
		}
		payload, err := json.Marshal(&config)
		assert.Nil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/config/", bytes.NewBuffer(payload))
		assert.Nil(t, err)
		req.AddCookie(cookie)
		req.Header.Set("content-type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		body, err := io.ReadAll(w.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "w3-theme-red")
	})
}
