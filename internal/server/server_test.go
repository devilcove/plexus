package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/plexus"
)

func TestServerLogs(t *testing.T) {
	user := plexus.User{
		Username: "hello",
		Password: "world",
	}
	createTestUser(t, user)
	setup(t)
	defer shutdown(t)

	t.Run("get", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/server/", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Server Logs")
	})

	t.Run("loglevel", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/server/logs/Debug", nil)
		r.AddCookie(testLogin(t, user))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		should.BeEqual(t, w.Result().StatusCode, http.StatusOK)
		body, err := io.ReadAll(w.Result().Body)
		should.NotBeError(t, err)
		should.ContainSubstring(t, string(body), "Server Logs")
	})
}

func TestServer(t *testing.T) {
	// skip for now: need a better way to test this
	t.Skip()
	setup(t)
	shutdown(t)

	t.Run("reset", func(t *testing.T) {
		go Run()
		time.Sleep(time.Second * 1)
		r := httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
		resp, err := http.DefaultClient.Do(r)
		should.NotBeError(t, err)
		should.NotBeNil(t, resp)
		defer resp.Body.Close()
		should.BeEqual(t, resp.StatusCode, http.StatusOK)
		p, err := os.FindProcess(os.Getpid())
		should.NotBeError(t, err)
		should.NotBeError(t, p.Signal(syscall.SIGHUP))
	})

	t.Run("brokerfail", func(t *testing.T) {
		t.Skip()
		go Run()
		r := httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
		resp, err := http.DefaultClient.Do(r)
		should.NotBeError(t, err)
		should.NotBeNil(t, resp)
		defer resp.Body.Close()
		should.BeEqual(t, resp.StatusCode, http.StatusOK)
		brokerfail <- 1
	})

	t.Run("webfail", func(t *testing.T) {
		t.Skip()
		go Run()
		time.Sleep(time.Second * 1)
		r := httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
		resp, err := http.DefaultClient.Do(r)
		should.NotBeError(t, err)
		should.NotBeNil(t, resp)
		defer resp.Body.Close()
		should.BeEqual(t, resp.StatusCode, http.StatusOK)
		webfail <- 1
	})
}
