package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/plexus"
)

func TestGetUsers(t *testing.T) {
	deleteAllUsers(t)
	admin := plexus.User{Username: "admin", Password: "pass", IsAdmin: true}
	user := plexus.User{Username: "test", Password: "pass", IsAdmin: false}
	createTestUser(t, admin)
	createTestUser(t, user)

	t.Run("admin", func(t *testing.T) {
		cookie := testLogin(t, admin)
		r := httptest.NewRequest(http.MethodGet, "/users/", nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		t.Log(w.Result())
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Authorized Users</h1>")
	})

	t.Run("user", func(t *testing.T) {
		cookie := testLogin(t, user)
		r := httptest.NewRequest(http.MethodGet, "/users/", nil)
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Edit User</h1>")
	})
}

func TestGetUser(t *testing.T) {
	deleteAllUsers(t)
	admin := plexus.User{Username: "admin", Password: "pass", IsAdmin: true}
	user := plexus.User{Username: "test", Password: "pass", IsAdmin: false}
	createTestUser(t, admin)
	createTestUser(t, user)

	t.Run("nosuchuser", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/user/user", nil)
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "no results found")
	})

	t.Run("user", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/user/admin", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusUnauthorized)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "need to be an admin to edit other users")
	})

	t.Run("admin", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/user/test", nil)
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Edit User</h1>")
	})
}

func TestAddUser(t *testing.T) {
	deleteAllUsers(t)
	admin := plexus.User{Username: "admin", Password: "pass", IsAdmin: true}
	user := plexus.User{Username: "test", Password: "pass", IsAdmin: false}
	createTestUser(t, admin)
	createTestUser(t, user)

	t.Run("displayAsUser", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/add", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusUnauthorized)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "admin rights required")
	})

	t.Run("displayAsAdmin", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/add", nil)
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>New User</h1>")
	})

	t.Run("addAsUser", func(t *testing.T) {
		payload := bodyParams("username", "test2", "password", "pass")
		r := httptest.NewRequest(http.MethodPost, "/users/add", payload)
		r.AddCookie(testLogin(t, user))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusUnauthorized)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "admin rights required")
	})

	t.Run("addAsAdmin", func(t *testing.T) {
		payload := bodyParams("username", "test2", "password", "pass")
		r := httptest.NewRequest(http.MethodPost, "/users/add", payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Authorized Users</h1>")
	})

	t.Run("userExists", func(t *testing.T) {
		payload := bodyParams("username", "test2", "password", "pass", "admin", "on")
		r := httptest.NewRequest(http.MethodPost, "/users/add", payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "user exists")
	})
}

func TestDeleteUser(t *testing.T) {
	deleteAllUsers(t)
	admin := plexus.User{Username: "admin", Password: "pass", IsAdmin: true}
	user := plexus.User{Username: "test", Password: "pass", IsAdmin: false}
	createTestUser(t, admin)
	createTestUser(t, user)

	t.Run("AsUser", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/users/admin", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusUnauthorized)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "admin rights required")
	})

	t.Run("NoSuchUser", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/users/notexit", nil)
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusNotFound)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "no results found")
	})

	t.Run("valid", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodDelete, "/users/test", nil)
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Authorized Users</h1>")
	})
}

func TestEditUser(t *testing.T) {
	deleteAllUsers(t)
	admin := plexus.User{Username: "admin", Password: "pass", IsAdmin: true}
	user := plexus.User{Username: "test", Password: "pass", IsAdmin: false}
	createTestUser(t, admin)
	createTestUser(t, user)

	t.Run("AsUserForOther", func(t *testing.T) {
		payload := bodyParams("password", "newpass")
		r := httptest.NewRequest(http.MethodPost, "/users/user/admin", payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusUnauthorized)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "admin rights required")
	})

	t.Run("AsUser", func(t *testing.T) {
		payload := bodyParams("password", "newpass")
		r := httptest.NewRequest(http.MethodPost, "/users/user/test", payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Edit User</h1>")
	})

	t.Run("blankPass", func(t *testing.T) {
		payload := bodyParams("password", "")
		r := httptest.NewRequest(http.MethodPost, "/users/user/test", payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "blank password")
	})

	t.Run("AsAdmin", func(t *testing.T) {
		payload := bodyParams("password", "newpass")
		r := httptest.NewRequest(http.MethodPost, "/users/user/test", payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "<h1>Authorized Users</h1>")
	})

	t.Run("NoSuchUser", func(t *testing.T) {
		payload := bodyParams("password", "newpass")
		r := httptest.NewRequest(http.MethodPost, "/users/user/notexist", payload)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.AddCookie(testLogin(t, admin))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusBadRequest)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "no results found")
	})
}
