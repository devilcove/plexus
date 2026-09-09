package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/devilcove/cookie"
	"github.com/devilcove/plexus"
)

const (
	cookieName = "plexus"
	cookieAge  = 86400 // one day
)

func InitializeSession() error {
	return cookie.New(cookieName, cookieAge)
}

func GetSessionData(r *http.Request) plexus.User {
	var user plexus.User
	bytes, err := cookie.Get(r, cookieName)
	if err != nil {
		slog.Error("get cookie", "error", err)
		return user
	}
	if err := json.Unmarshal(bytes, &user); err != nil {
		slog.Error("decode user", "error", err)
	}
	return user
}

func ClearSession(w http.ResponseWriter) {
	if err := cookie.Clear(w, cookieName, false); err != nil {
		slog.Error("save session", "error", err)
	}
}

func saveSession(w http.ResponseWriter, data any) {
	bytes, err := json.Marshal(data)
	if err != nil {
		slog.Error("encode cookie data", "error", err)
	}
	if err := cookie.Save(w, cookieName, bytes); err != nil {
		slog.Error("save cookie", "error", err)
	}
}
