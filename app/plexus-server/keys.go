package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"sync"
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
	key.Value, err = newValue(key.Name)
	if err != nil {
		processError(c, http.StatusInternalServerError, "unable to encode key"+err.Error())
		return
	}
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
	keyid := c.Param("id")
	key, err := boltdb.Get[plexus.Key](keyid, "keys")
	if err != nil {
		processError(c, http.StatusBadRequest, "key does not exist")
		return
	}
	if err := removeKey(key); err != nil {
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

func newValue(name string) (string, error) {
	device, err := nkeys.CreateUser()
	if err != nil {
		return "", err
	}
	seed, err := device.Seed()
	if err != nil {
		return "", err
	}
	keyValue := plexus.KeyValue{
		URL:     "nats://" + config.FQDN + ":4222",
		Seed:    string(seed),
		KeyName: name,
	}
	payload, err := json.Marshal(&keyValue)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(payload), nil
}

func decrementKeyUsage(name string) (plexus.Key, error) {
	key, err := boltdb.Get[plexus.Key](name, "keys")
	if err != nil {
		return key, err
	}
	if key.Usage == 1 {
		removeKey(key)
		return key, nil
	}
	key.Usage = key.Usage - 1
	if err := boltdb.Save(key, key.Name, "keys"); err != nil {
		return key, err
	}
	return key, nil
}

func expireKeys(ctx context.Context, wg *sync.WaitGroup) {
	slog.Debug("key expiry checks starting")
	defer wg.Done()
	ticker := time.NewTicker(time.Hour * 6)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			slog.Debug("checking for expired keys")
			keys, err := boltdb.GetAll[plexus.Key]("keys")
			if err != nil {
				slog.Error("get keys", "error", err)
				continue
			}
			for _, key := range keys {
				if key.Expires.Before(time.Now()) {
					slog.Debug("key has expired ...deleting", "key", key.Name, "expiry time", key.Expires.Format(time.RFC822))
					removeKey(key)
				}
			}
		}
	}
}

func removeKey(key plexus.Key) error {
	var errs error
	if err := boltdb.Delete[plexus.Key](key.Name, "keys"); err != nil {
		slog.Error("delete key from db", "error", err)
		errors.Join(errs, err)
	}
	token, err := plexus.DecodeToken(key.Value)
	if err != nil {
		errs = errors.Join(errs, err)
		return errs
	}
	pk := createNkeyUser(token.Seed)
	for i, nkey := range natsOptions.Nkeys {
		if nkey == nil {
			continue
		}
		if nkey.Nkey == pk.Nkey {
			natsOptions.Nkeys = slices.Delete(natsOptions.Nkeys, i, i+1)
		}
	}
	if err := natServer.ReloadOptions(natsOptions); err != nil {
		errs = errors.Join(errs, err)
	}
	return errs
}
