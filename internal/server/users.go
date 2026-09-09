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
	if err = store.Save(&plexus.User{
		Username: user,
		Password: password,
		IsAdmin:  true,
		Updated:  time.Now(),
	}, user, userBucket); err != nil {
		slog.Error("create default user", "error", err)
		return err
	}
	slog.Info("default user created")
	return nil
}

func adminExist() bool {
	var user plexus.User
	var found bool
	db := store.Connection()
	if err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
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
	session := GetSessionData(r)
	if !session.IsAdmin {
		getCurrentUser(w, r)
		return
	}
	slog.Debug("getting users", "admin", session.IsAdmin)
	users, err := store.GetAll[plexus.User](userBucket)
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
	render(w, "users", returnedUsers)
}

func getUser(w http.ResponseWriter, r *http.Request) {
	session := GetSessionData(r)
	userToEdit := r.PathValue("name")
	slog.Debug(
		"getUser",
		"admin",
		session.IsAdmin,
		"visitor",
		session.Username,
		"to edit",
		userToEdit,
	)
	if !session.IsAdmin && session.Username != userToEdit {
		processError(w, http.StatusUnauthorized, "you need to be an admin to edit other users")
		return
	}
	user, err := store.Get[plexus.User](userToEdit, userBucket)
	if err != nil {
		processError(w, http.StatusBadRequest, err.Error())
		return
	}
	user.Password = ""
	render(w, "editUser", user)
}

func getCurrentUser(w http.ResponseWriter, r *http.Request) {
	slog.Debug("get current user")
	data := GetSessionData(r)
	user, err := store.Get[plexus.User](data.Username, userBucket)
	if err != nil {
		processError(w, http.StatusBadRequest, "no such user "+err.Error())
		return
	}
	render(w, "editUser", user)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	session := GetSessionData(r)
	if !session.IsAdmin {
		processError(w, http.StatusUnauthorized, "admin rights required")
		return
	}
	user := r.PathValue("name")
	if err := store.Delete(user, userBucket); err != nil {
		processError(w, http.StatusNotFound, err.Error())
		return
	}
	getUsers(w, r)
}

func displayAddUser(w http.ResponseWriter, r *http.Request) {
	session := GetSessionData(r)
	if !session.IsAdmin {
		processError(w, http.StatusUnauthorized, "admin rights required")
		return
	}
	render(w, "newUser", nil)
}

func addUser(w http.ResponseWriter, r *http.Request) {
	session := GetSessionData(r)
	if !session.IsAdmin {
		processError(w, http.StatusUnauthorized, "admin rights required")
		return
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
	if _, err := store.Get[plexus.User](user.Username, userBucket); err == nil {
		processError(w, http.StatusBadRequest, "user exists")
		return
	}
	slog.Info("saving new user", "user", user)
	if err := store.Save(user, user.Username, userBucket); err != nil {
		processError(w, http.StatusInternalServerError, "unable to save user "+err.Error())
		return
	}
	getUsers(w, r)
}

func editUser(w http.ResponseWriter, r *http.Request) {
	input := r.FormValue("password")
	if input == "" {
		processError(w, http.StatusBadRequest, "blank password")
		return
	}
	data := GetSessionData(r)
	userToEdit := r.PathValue("name")
	if !data.IsAdmin && data.Username != userToEdit {
		processError(w, http.StatusUnauthorized, "admin rights required to update other users")
		return
	}
	user, err := store.Get[plexus.User](userToEdit, userBucket)
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
	if err := store.Save(user, user.Username, userBucket); err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	getUsers(w, r)
}
