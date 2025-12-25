package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func checkDefaultUser(user, pass string) error {
	if adminExist() {
		slog.Debug("admin exists")
		return nil
	}
	password, err := hashPassword(pass)
	if err != nil {
		slog.Error("hash error", "error", err)
		return err
	}
	if err = boltdb.Save(&plexus.User{
		Username: user,
		Password: password,
		IsAdmin:  true,
		Updated:  time.Now(),
	}, user, userTable); err != nil {
		slog.Error("create default user", "error", err)
		return err
	}
	slog.Info("default user created")
	return nil
}

func adminExist() bool {
	var user plexus.User
	var found bool
	db := boltdb.Connection()
	if err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(userTable))
		if b == nil {
			return boltdb.ErrNoResults
		}
		_ = b.ForEach(func(_, v []byte) error {
			if err := json.Unmarshal(v, &user); err != nil {
				return err
			}
			if user.IsAdmin {
				found = true
			}
			return nil
		})
		return nil
	}); err != nil {
		return false
	}
	return found
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	return string(bytes), err
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	session := GetSession(w, r)
	if session == nil {
		displayLogin(w, r)
		return
	}
	if !session.Admin {
		getCurrentUser(w, r)
		return
	}
	slog.Debug("getting uses", "admin", session.Admin)
	users, err := boltdb.GetAll[plexus.User](userTable)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(users) == 0 {
		processError(w, http.StatusFailedDependency, "no users")
		return
	}
	returnedUsers := []plexus.User{}
	for _, user := range users {
		user.Password = ""
		returnedUsers = append(returnedUsers, user)
	}
	if err := session.Session.Save(r, w); err != nil {
		slog.Error("save session", "error", err)
	}
	if err := templates.ExecuteTemplate(w, "users", returnedUsers); err != nil {
		slog.Error("template execute", "template", "users", "data", returnedUsers, "error", err)
	}
}

func getUser(w http.ResponseWriter, r *http.Request) {
	session := GetSession(w, r)
	if session == nil {
		displayLogin(w, r)
		return
	}
	userToEdit := r.PathValue("name")
	slog.Debug("getUser", "admin", session.Admin, "visitor", session.User, "to edit", userToEdit)
	if !session.Admin && session.User != userToEdit {
		processError(w, http.StatusUnauthorized, "you need to be an admin to edit other users")
		return
	}
	user, err := boltdb.Get[plexus.User](userToEdit, userTable)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	user.Password = ""
	if err := session.Session.Save(r, w); err != nil {
		slog.Error("session save", "error", err)
	}
	if err := templates.ExecuteTemplate(w, "editUser", user); err != nil {
		slog.Error("template execute", "template", "editUser", "data", user, "error", err)
	}
}

func getCurrentUser(w http.ResponseWriter, r *http.Request) {
	slog.Debug("get current user")
	session := GetSession(w, r)
	if session == nil {
		displayLogin(w, r)
		return
	}
	user, err := boltdb.Get[plexus.User](session.User, userTable)
	if err != nil {
		processError(w, http.StatusBadRequest, "no such user "+err.Error())
		return
	}
	if err := session.Session.Save(r, w); err != nil {
		slog.Error("session save", "error", err)
	}
	if err := templates.ExecuteTemplate(w, "editUser", user); err != nil {
		slog.Error("template execute", "template", "editUser", "data", user, "error", err)
	}
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	session := GetSession(w, r)
	if session == nil {
		displayLogin(w, r)
		return
	}
	if !session.Admin {
		processError(w, http.StatusUnauthorized, "admin rights required")
		return
	}
	user := r.PathValue("name")
	if err := boltdb.Delete[plexus.User](user, userTable); err != nil {
		processError(w, http.StatusFailedDependency, err.Error())
		return
	}
	getUsers(w, r)
}

func displayAddUser(w http.ResponseWriter, r *http.Request) {
	session := GetSession(w, r)
	if !session.Admin {
		processError(w, http.StatusUnauthorized, "admin rights required")
		return
	}
	if err := templates.ExecuteTemplate(w, "newUser", nil); err != nil {
		slog.Error("template execute", "template", "newUser", "data", "nil", "error", err)
	}
}

func addUser(w http.ResponseWriter, r *http.Request) {
	session := GetSession(w, r)
	if session == nil {
		displayLogin(w, r)
		return
	}
	if !session.Admin {
		processError(w, http.StatusUnauthorized, "admin rights required")
		return
	}
	if err := r.ParseForm(); err != nil {
		processError(w, http.StatusBadRequest, "invalid form")
	}
	user := plexus.User{
		Username: r.FormValue("username"),
	}
	password, err := hashPassword(r.FormValue("password"))
	if err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	user.Password = password
	if r.FormValue("admin") == "on" {
		user.IsAdmin = true
	}
	if _, err := boltdb.Get[plexus.User](user.Username, userTable); err == nil {
		processError(w, http.StatusBadRequest, "user exists")
		return
	}
	slog.Info("saving new user", "user", user)
	if err := boltdb.Save(user, user.Username, userTable); err != nil {
		processError(w, http.StatusInternalServerError, "unable to save user "+err.Error())
		return
	}
	getUsers(w, r)
}

func editUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		processError(w, http.StatusBadRequest, "invalid form")
		return
	}
	input := r.FormValue("password")
	if input == "" {
		processError(w, http.StatusBadRequest, "blank password")
		return
	}
	session := GetSession(w, r)
	if session == nil {
		displayLogin(w, r)
		return
	}
	userToEdit := r.PathValue("name")
	if !session.Admin && session.User != userToEdit {
		processError(w, http.StatusUnauthorized, "admin right required to update other users")
		return
	}
	user, err := boltdb.Get[plexus.User](userToEdit, userTable)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	password, err := hashPassword(input)
	if err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	user.Password = password
	if err := boltdb.Save(user, user.Username, userTable); err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	getUsers(w, r)
}
