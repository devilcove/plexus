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

func TestDefaultUser(t *testing.T) {
	t.Run("noadmim", func(t *testing.T) {
		err := deleteAllUsers(true)
		should.BeNil(t, err)
		err = checkDefaultUser("admin", "pass")
		should.BeNil(t, err)
		user, err := boltdb.Get[plexus.User]("admin", userTable)
		should.BeNil(t, err)
		should.BeEqual(t, user.Username, "admin")
		should.BeTrue(t, user.IsAdmin)
	})
	t.Run("env", func(t *testing.T) {
		err := deleteAllUsers(true)
		should.BeNil(t, err)
		err = checkDefaultUser("Administrator", "password")
		should.BeNil(t, err)
		user, err := boltdb.Get[plexus.User]("Administrator", userTable)
		should.BeNil(t, err)
		should.BeEqual(t, user.Username, "Administrator")
		should.BeTrue(t, user.IsAdmin)
	})
	t.Run("adminexists", func(t *testing.T) {
		err := checkDefaultUser("Administator", "password")
		should.BeNil(t, err)
		user, err := boltdb.Get[plexus.User]("Administrator", userTable)
		should.BeNil(t, err)
		should.BeEqual(t, user.Username, "Administrator")
		should.BeTrue(t, user.IsAdmin)
	})
}

func TestAuthFail(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/server/", nil)
	should.BeNil(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	should.BeEqual(t, w.Code, http.StatusOK)
	body, err := io.ReadAll(w.Body)
	should.BeNil(t, err)
	should.ContainSubstring(t, string(body), "<h1>Login</h1")
}
