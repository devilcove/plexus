package main

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/plexus"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nkeys"
)

func displayCreateKey(c *gin.Context) {
	session := sessions.Default(c)
	page := getPage(session.Get("user"))
	page.Page = "addKey"
	c.HTML(http.StatusOK, "addKey", page)

}

func addKey(c *gin.Context) {
	var err error
	key := plexus.Key{}
	if err := c.Bind(&key); err != nil {
		processError(c, http.StatusBadRequest, "invalid key data")
		return
	}
	key.Expires, err = time.Parse("2006-01-02", key.DispExp)
	if err != nil {
		processError(c, http.StatusBadRequest, "invalid key "+err.Error())
		return
	}
	if err := validateKey(key); err != nil {
		processError(c, http.StatusBadRequest, "invalid key "+err.Error())
		return
	}
	key.Value = newValue()
	newDevice <- key.Value
	if key.Usage == 0 {
		key.Usage = 1
	}
	if key.Expires.IsZero() {
		key.Expires = time.Now().Add(24 * time.Hour)
	}
	existing, err := boltdb.Get[plexus.Key](key.Name, "keys")
	if err != nil && !errors.Is(err, boltdb.ErrNoResults) {
		processError(c, http.StatusInternalServerError, "retrieve key"+err.Error())
		return
	}
	if existing.Name != "" {
		processError(c, http.StatusBadRequest, "key exists with name:"+existing.Name)
		return
	}
	if err := boltdb.Save(key, key.Name, "keys"); err != nil {
		processError(c, http.StatusInternalServerError, "saving key "+err.Error())
		return
	}
	displayKeys(c)
}

func displayKeys(c *gin.Context) {
	keys, err := boltdb.GetAll[plexus.Key]("keys")
	if err != nil {
		processError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.HTML(http.StatusOK, "keys", keys)

}

func deleteKey(c *gin.Context) {
	key := c.Param("id")
	if err := boltdb.Delete[plexus.Key](key, "keys"); err != nil {
		if errors.Is(err, boltdb.ErrNoResults) {
			processError(c, http.StatusBadRequest, "key does not exist")
			return
		}
		processError(c, http.StatusInternalServerError, "delete key "+err.Error())
		return
	}
	displayKeys(c)
}

func validateKey(key plexus.Key) error {
	if len(key.Name) > 255 {
		return errors.New("too long")
	}
	if !regexp.MustCompile(`^[a-z,-]+$`).MatchString(key.Name) {
		return errors.New("invalid chars")
	}
	return nil
}

func newValue() string {
	device, err := nkeys.CreateUser()
	if err != nil {
		slog.Error("key value", "error", err)
	}
	seed, err := device.Seed()
	if err != nil {
		slog.Error("key seed", "error", err)
	}
	return string(seed)
}
