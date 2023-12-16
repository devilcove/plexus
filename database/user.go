package database

import (
	"encoding/json"

	"github.com/devilcove/plexus"
	"go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
)

func SaveUser(u *plexus.User) error {
	value, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(USERS_TABLE_NAME))
		return b.Put([]byte(u.Username), value)
	})
}

func GetUser(name string) (plexus.User, error) {
	user := plexus.User{}
	if err := db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte(USERS_TABLE_NAME)).Get([]byte(name))
		if v == nil {
			return ErrNoResults
		}
		if err := json.Unmarshal(v, &user); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return user, err
	}
	return user, nil
}

func GetAllUsers() ([]plexus.User, error) {
	var users []plexus.User
	var user plexus.User
	if err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(USERS_TABLE_NAME))
		if b == nil {
			return ErrNoResults
		}
		_ = b.ForEach(func(k, v []byte) error {
			if err := json.Unmarshal(v, &user); err != nil {
				return err
			}
			users = append(users, user)
			return nil
		})
		return nil
	}); err != nil {
		return users, err
	}
	return users, nil
}

func DeleteUser(name string) error {
	if _, err := GetUser(name); err != nil {
		return err
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(USERS_TABLE_NAME)).Delete([]byte(name))
	}); err != nil {
		return err
	}
	return nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	return string(bytes), err
}
