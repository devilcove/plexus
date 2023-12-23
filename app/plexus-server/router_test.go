package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/stretchr/testify/assert"
)

func TestDefaultUser(t *testing.T) {
	t.Run("noadmim", func(t *testing.T) {
		err := deleteAllUsers(true)
		assert.Nil(t, err)
		checkDefaultUser()
		user, err := boltdb.Get[plexus.User]("admin", "users")
		assert.Nil(t, err)
		assert.Equal(t, "admin", user.Username)
		assert.Equal(t, true, user.IsAdmin)
	})
	t.Run("env", func(t *testing.T) {
		err := deleteAllUsers(true)
		assert.Nil(t, err)
		err = os.Setenv("PLEXUS_USER", "Administrator")
		assert.Nil(t, err)
		checkDefaultUser()
		user, err := boltdb.Get[plexus.User]("Administrator", "users")
		assert.Nil(t, err)
		assert.Equal(t, "Administrator", user.Username)
		assert.Equal(t, true, user.IsAdmin)
	})
	t.Run("adminexists", func(t *testing.T) {
		checkDefaultUser()
		user, err := boltdb.Get[plexus.User]("Administrator", "users")
		assert.Nil(t, err)
		assert.Equal(t, "Administrator", user.Username)
		assert.Equal(t, true, user.IsAdmin)
	})
}

func TestAuthFail(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/config/", nil)
	assert.Nil(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body, err := io.ReadAll(w.Body)
	assert.Nil(t, err)
	assert.Contains(t, string(body), "<h1>Login</h1")
}
