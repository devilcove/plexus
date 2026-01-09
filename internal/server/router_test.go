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
		deleteAllUsers(t)
		err := checkDefaultUser("admin", "pass")
		should.NotBeError(t, err)
		user, err := boltdb.Get[plexus.User]("admin", userTable)
		should.NotBeError(t, err)
		should.BeEqual(t, user.Username, "admin")
		should.BeTrue(t, user.IsAdmin)
	})
	t.Run("env", func(t *testing.T) {
		deleteAllUsers(t)
		err := checkDefaultUser("Administrator", "password")
		should.NotBeError(t, err)
		user, err := boltdb.Get[plexus.User]("Administrator", userTable)
		should.NotBeError(t, err)
		should.BeEqual(t, user.Username, "Administrator")
		should.BeTrue(t, user.IsAdmin)
	})
	t.Run("adminexists", func(t *testing.T) {
		err := checkDefaultUser("Administator", "password")
		should.NotBeError(t, err)
		user, err := boltdb.Get[plexus.User]("Administrator", userTable)
		should.NotBeError(t, err)
		should.BeEqual(t, user.Username, "Administrator")
		should.BeTrue(t, user.IsAdmin)
	})
}

func TestAuthFail(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/server/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	should.BeEqual(t, w.Code, http.StatusUnauthorized)
	body, err := io.ReadAll(w.Body)
	should.NotBeError(t, err)
	should.ContainSubstring(t, string(body), "Unauthorized")
}
