package server

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/devilcove/plexus"
	"github.com/gorilla/sessions"
)

const (
	cookieAge   = 300
	sessionName = "plexus"
)

var (
	store              *sessions.CookieStore
	sessionInitialized bool
)

// Session represents a user session.
type Session struct {
	User     string
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
	store.MaxAge(cookieAge)
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

func GetSession(_ http.ResponseWriter, r *http.Request) *Session {
	sess := &Session{}
	session, err := store.Get(r, sessionName)
	if err != nil {
		slog.Error("session err", "error", err)
		return nil
	}
	user := session.Values["user"]
	if u, ok := user.(string); ok {
		sess.User = u
	}
	loggedIn := session.Values["loggedIn"]
	if l, ok := loggedIn.(bool); ok {
		sess.LoggedIn = l
	}
	admin := session.Values["admin"]
	if a, ok := admin.(bool); ok {
		sess.Admin = a
	}

	page := session.Values["page"]
	if p, ok := page.(string); ok {
		sess.Page = p
	}
	sess.Session = session
	return sess
}

func ClearSession(w http.ResponseWriter, r *http.Request) {
	session := GetSession(w, r)
	if session == nil {
		s := sessions.NewSession(store, sessionName)
		s.Options.MaxAge = -1
		if err := s.Save(r, w); err != nil {
			slog.Error("save session", "error", err)
		}
		return
	}
	session.Session.Options.MaxAge = -1
	if err := session.Session.Save(r, w); err != nil {
		slog.Error("save session", "error", err)
	}
}

func NewSession(
	w http.ResponseWriter,
	r *http.Request,
	user plexus.User,
	loggedIn bool,
	page string,
) {
	session := sessions.NewSession(store, sessionName)
	session.Values["username"] = user.Username
	session.Values["admin"] = user.IsAdmin
	session.Values["loggedIn"] = loggedIn
	session.Values["page"] = page
	session.Options = store.Options
	if err := session.Save(r, w); err != nil {
		slog.Error("save session", "error", err)
	}
	slog.Info("new session created")
}

func (s *Session) Save(w http.ResponseWriter, r *http.Request) {
	if err := s.Session.Save(r, w); err != nil {
		buf := bytes.Buffer{}
		l := log.New(&buf, "", log.Lshortfile)
		_ = l.Output(2, fmt.Sprintf("%s %s %s", "session save", "error", err))
		slog.Error(buf.String())
	}
}
