package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func checkDefaultUser() {
	if adminExist() {
		slog.Debug("admin exists")
		return
	}
	user, ok := os.LookupEnv("PLEXUS_USER")
	if !ok {
		user = "admin"
	}
	pass, ok := os.LookupEnv("PLEXUS_PASS")
	if !ok {
		pass = "password"
	}
	password, err := hashPassword(pass)
	if err != nil {
		slog.Error("hash error", "error", err)
		return
	}
	if err = boltdb.Save(&plexus.User{
		Username: user,
		Password: password,
		IsAdmin:  true,
		Updated:  time.Now(),
	}, user, "users"); err != nil {
		slog.Error("create default user", "error", err)
		return
	}
	slog.Info("default user created")
}

func adminExist() bool {
	var user plexus.User
	var found bool
	db := boltdb.Connection()
	if err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("users"))
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
