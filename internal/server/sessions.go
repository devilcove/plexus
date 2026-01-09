package server

import (
	"crypto/rand"
	"encoding/gob"
	"errors"
	"log/slog"
	"net/http"

	"github.com/devilcove/plexus"
	"github.com/gorilla/sessions"
)

const (
	cookieName = "plexus"
	dataName   = "data"
)

var (
	store              *sessions.CookieStore
	sessionInitialized bool
	ErrNotInitialized  = errors.New("session is not initialized")
)

// Session represents a user session.
type Session struct {
	UserName string
	LoggedIn bool
	Admin    bool
	Page     string
	Session  *sessions.Session
}

func InitializeSession() {
	if sessionInitialized {
		slog.Error("session already initialized")
		return
	}
	store = sessions.NewCookieStore(keypairs())
	store.Options.HttpOnly = true
	store.Options.SameSite = http.SameSiteStrictMode
}

func keypairs() ([]byte, []byte) {
	buf1 := make([]byte, 32)
	buf2 := make([]byte, 32)
	rand.Read(buf1)
	rand.Read(buf2)
	return buf1, buf2
}

func GetSessionData(r *http.Request) plexus.User {
	s := GetSession(r)
	data, ok := s.Values[dataName].(plexus.User)
	if !ok {
		data = plexus.User{}
	}
	return data
}

func GetSession(r *http.Request) *sessions.Session {
	s, err := store.Get(r, cookieName)
	if err != nil {
		s = sessions.NewSession(store, cookieName)
	}
	return s
}

func ClearSession(w http.ResponseWriter, r *http.Request) {
	s := sessions.NewSession(store, cookieName)
	s.Options = store.Options
	s.Options.MaxAge = -1
	if err := s.Save(r, w); err != nil {
		slog.Error("save session", "error", err)
	}
}

func saveSession(w http.ResponseWriter, r *http.Request, data any) {
	s := sessions.NewSession(store, cookieName)
	s.Options = store.Options
	s.Options.MaxAge = 0 // session cookie
	gob.Register(data)
	s.Values[dataName] = data
	if err := s.Save(r, w); err != nil {
		slog.Error("save session", "error", err)
	}
}
