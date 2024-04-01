package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
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
		_ = b.ForEach(func(k, v []byte) error {
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

func getUsers(c *gin.Context) {
	session := sessions.Default(c)
	admin := session.Get("admin").(bool)
	slog.Debug("getUsers", "admin", admin, "ad", session.Get("admin"), "visitor", session.Get("user"))
	if !admin {
		getCurrentUser(c)
		return
	}
	slog.Debug("getting uses", "admin", admin)
	users, err := boltdb.GetAll[plexus.User](userTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(users) == 0 {
		processError(c, http.StatusFailedDependency, "no users")
		return
	}
	returnedUsers := []plexus.User{}
	for _, user := range users {
		user.Password = ""
		returnedUsers = append(returnedUsers, user)
	}
	_ = session.Save()
	c.HTML(http.StatusOK, "users", returnedUsers)
}

func getUser(c *gin.Context) {
	session := sessions.Default(c)
	visitor := session.Get("user").(string)
	admin := session.Get("admin").(bool)
	userToEdit := c.Param("name")
	slog.Debug("getUser", "admin", admin, "visitor", visitor, "to edit", userToEdit)
	if !admin && visitor != userToEdit {
		processError(c, http.StatusUnauthorized, "you need to be an admin to edit other users")
		return
	}
	user, err := boltdb.Get[plexus.User](userToEdit, userTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	user.Password = ""
	_ = session.Save()
	c.HTML(http.StatusOK, "editUser", user)
}

func getCurrentUser(c *gin.Context) {
	slog.Debug("get current user")
	session := sessions.Default(c)
	visitor := session.Get("user").(string)
	user, err := boltdb.Get[plexus.User](visitor, userTable)
	if err != nil {
		processError(c, http.StatusBadRequest, "no such user "+err.Error())
		return
	}
	_ = session.Save()
	c.HTML(http.StatusOK, "editUser", user)
}

func deleteUser(c *gin.Context) {
	session := sessions.Default(c)
	admin := session.Get("admin").(bool)
	if !admin {
		processError(c, http.StatusUnauthorized, "admin rights required")
		return
	}
	user := c.Param("name")
	if err := boltdb.Delete[plexus.User](user, userTable); err != nil {
		processError(c, http.StatusFailedDependency, err.Error())
		return
	}
	getUsers(c)
}

func displayAddUser(c *gin.Context) {
	session := sessions.Default(c)
	if !session.Get("admin").(bool) {
		processError(c, http.StatusUnauthorized, "admin rights required")
		return
	}
	c.HTML(http.StatusOK, "newUser", nil)
}

func addUser(c *gin.Context) {
	input := struct {
		Username string
		Password string
		Admin    string
	}{}
	session := sessions.Default(c)
	if !session.Get("admin").(bool) {
		processError(c, http.StatusUnauthorized, "admin rights required")
		return
	}
	if err := c.Bind(&input); err != nil {
		processError(c, http.StatusBadRequest, "invalid user data")
		return
	}
	password, err := hashPassword(input.Password)
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	user := plexus.User{
		Username: input.Username,
		Password: password,
		Updated:  time.Now(),
	}
	if input.Admin == "on" {
		user.IsAdmin = true
	}
	if _, err := boltdb.Get[plexus.User](user.Username, userTable); err == nil {
		processError(c, http.StatusBadRequest, "user exists")
		return
	}
	slog.Debug("saving new user", "user", user)
	if err := boltdb.Save(user, input.Username, userTable); err != nil {
		processError(c, http.StatusInternalServerError, "unable to save user "+err.Error())
		return
	}
	getUsers(c)
}

func editUser(c *gin.Context) {
	input := struct {
		Password string
	}{}
	if err := c.Bind(&input); err != nil {
		processError(c, http.StatusBadRequest, "invalid user data")
		return
	}
	session := sessions.Default(c)
	admin := session.Get("admin").(bool)
	visitor := session.Get("user").(string)
	userToEdit := c.Param("name")
	if !admin && visitor != userToEdit {
		processError(c, http.StatusUnauthorized, "admin right required to update other users")
		return
	}
	user, err := boltdb.Get[plexus.User](userToEdit, userTable)
	if err != nil {
		processError(c, http.StatusBadRequest, err.Error())
		return
	}
	password, err := hashPassword(input.Password)
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	user.Password = password
	if err := boltdb.Save(user, user.Username, userTable); err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	getUsers(c)
}
