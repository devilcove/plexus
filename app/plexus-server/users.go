package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
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

func getNatsUsers(c *gin.Context) {
	nats := []plexus.NatsUser{}
	peers, _ := boltdb.GetAll[plexus.Peer](peerTable)

	for _, nkey := range natsOptions.Nkeys {
		//fmt.Println(nkey.Permissions.Publish.Allow)
		if slices.Contains(nkey.Permissions.Subscribe.Allow, "networks.>") {
			user := plexus.NatsUser{
				Kind:      "plexus-agent",
				Subscribe: []string{"networks.>", "<id>"},
				Publish:   []string{"checkin.<id>", "<id>"},
			}
			for _, peer := range peers {
				if peer.PubNkey == nkey.Nkey {
					user.Name = peer.Name
				}
			}
			nats = append(nats, user)
			continue
		}
		if slices.Contains(nkey.Permissions.Publish.Allow, ">") {
			user := plexus.NatsUser{
				Kind:      "server",
				Name:      "-",
				Subscribe: []string{"any"},
				Publish:   []string{"any"},
			}
			nats = append(nats, user)
			continue
		}
		if slices.Contains(nkey.Permissions.Publish.Allow, "register") {
			user := plexus.NatsUser{
				Kind:      "registation key",
				Name:      "-",
				Subscribe: []string{"-"},
				Publish:   []string{"register"},
			}
			nats = append(nats, user)
		}
	}
	c.HTML(http.StatusOK, "natsUsers", nats)
}
