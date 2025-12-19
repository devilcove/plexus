package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
	"github.com/devilcove/mux"
	"github.com/devilcove/plexus"
)

var router *mux.Router

func TestMain(m *testing.M) {
	if _, err := os.Stat("./test.db"); err == nil {
		if err := os.Remove("./test.db"); err != nil {
			log.Println("remove db", err)
			os.Exit(1)
		}
	}
	if err := boltdb.Initialize("./test.db",
		[]string{userTable, keyTable, networkTable, peerTable, settingTable, "keypairs"},
	); err != nil {
		log.Println("init db", err)
		os.Exit(2)
	}
	// 	wg := &sync.WaitGroup{}
	// 	ctx, cancel := context.WithCancel(context.Background())
	// 	newDevice = make(chan string, 1)
	// 	wg.Add(1)
	// 	go broker(ctx, wg, nil)
	plexus.SetLogging("debug")
	router = setupRouter(slog.Default())
	code := m.Run()
	// 	cancel()
	// 	wg.Wait()
	boltdb.Close()
	os.Exit(code)
}

func TestDisplayMainPage(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	should.BeNil(t, err)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	body, err := io.ReadAll(w.Body)
	should.BeNil(t, err)
	should.BeEqual(t, w.Code, http.StatusOK)
	should.ContainSubstring(t, string(body), "<title>Plexus</title>")
}

func TestGetPage(t *testing.T) {
	// no user
	page := getPage("someone")
	should.BeEqual(t, page.Version, version)
}

func TestLogin(t *testing.T) {
	err := deleteAllUsers(true)
	should.BeNil(t, err)
	t.Run("nousers", func(t *testing.T) {
		form := url.Values{}
		form.Add("username", "admin")
		form.Add("password", "testing")
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		should.BeNil(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "invalid user")
	})
	err = createTestUser(plexus.User{
		Username: "testing",
		Password: "testing",
	})
	should.BeNil(t, err)
	t.Run("wronguser", func(t *testing.T) {
		user := plexus.User{
			Username: "admin",
			Password: "testing",
		}
		payload, err := json.Marshal(&user)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		should.BeNil(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "invalid user")
	})
	t.Run("wrongpass", func(t *testing.T) {
		user := plexus.User{
			Username: "testing",
			Password: "testing2",
		}
		payload, err := json.Marshal(&user)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		should.BeNil(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusBadRequest)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "invalid user")
	})
	t.Run("valid", func(t *testing.T) {
		user := plexus.User{
			Username: "testing",
			Password: "testing",
		}
		payload, err := json.Marshal(&user)
		should.BeNil(t, err)
		req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		should.BeNil(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		should.BeEqual(t, w.Code, http.StatusOK)
		body, err := io.ReadAll(w.Body)
		should.BeNil(t, err)
		should.ContainSubstring(t, string(body), "<title>Plexus</title>")
		should.NotBeNil(t, w.Result().Cookies())
	})
}

func TestLogout(t *testing.T) {
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/logout", nil)
	should.BeNil(t, err)
	router.ServeHTTP(w, req)
	should.BeEqual(t, w.Code, http.StatusOK)
	should.BeEqual(t, w.Result().Cookies(), []*http.Cookie{})
}

func deleteAllUsers(deleteAll bool) (errs error) {
	users, err := boltdb.GetAll[plexus.User](userTable)
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Username != "admin" || deleteAll == true {
			if err := boltdb.Delete[plexus.User](user.Username, userTable); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	return errs
}

func testLogin(data plexus.User) (*http.Cookie, error) {
	w := httptest.NewRecorder()
	form := url.Values{}
	form.Add("username", data.Username)
	form.Add("password", data.Password)
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "plexus" {
			return cookie, nil
		}
	}
	return nil, errors.New("no cookie")
}

func createTestUser(user plexus.User) error {
	user.Password, _ = hashPassword(user.Password)
	if err := boltdb.Save(&user, user.Username, userTable); err != nil {
		return err
	}
	return nil
}
