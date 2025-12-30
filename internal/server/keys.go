package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/devilcove/boltdb"
	"github.com/devilcove/configuration"
	"github.com/devilcove/plexus"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
)

func displayCreateKey(w http.ResponseWriter, r *http.Request) {
	session := GetSession(w, r)
	if session == nil {
		displayLogin(w, r)
		return
	}
	page := getPage(session.UserName)
	page.Page = "addKey"
	if err := templates.ExecuteTemplate(w, "addKey", page); err != nil {
		slog.Error("execute template", "template", "addKey", "page", page, "error", err)
	}
}

func addKey(w http.ResponseWriter, r *http.Request) {
	var err error
	usage, err := strconv.Atoi(r.FormValue("usage"))
	if err != nil {
		usage = 1
	}
	key := plexus.Key{
		Name:    r.FormValue("name"),
		Usage:   usage,
		DispExp: r.FormValue("expires"),
	}
	key.Expires, err = time.Parse("2006-01-02", key.DispExp)
	if err != nil {
		processError(w, http.StatusBadRequest, "invalid key "+err.Error())
		return
	}
	if err := validateKey(key); err != nil {
		processError(w, http.StatusBadRequest, "invalid key "+err.Error())
		return
	}
	key.Value, err = newValue(key.Name)
	if err != nil {
		processError(w, http.StatusInternalServerError, "unable to encode key"+err.Error())
		return
	}
	if err := addDevice(key.Value); err != nil {
		processError(w, http.StatusInternalServerError, "unable to add device"+err.Error())
	}
	if key.Usage == 0 {
		key.Usage = 1
	}
	if key.Expires.IsZero() {
		key.Expires = time.Now().Add(keyExpiry)
	}
	existing, err := boltdb.Get[plexus.Key](key.Name, keyTable)
	if err != nil && !errors.Is(err, boltdb.ErrNoResults) {
		processError(w, http.StatusInternalServerError, "retrieve key"+err.Error())
		return
	}
	if existing.Name != "" {
		processError(w, http.StatusBadRequest, "key exists with name:"+existing.Name)
		return
	}
	if err := boltdb.Save(key, key.Name, keyTable); err != nil {
		processError(w, http.StatusInternalServerError, "saving key "+err.Error())
		return
	}
	displayKeys(w, r)
}

func displayKeys(w http.ResponseWriter, _ *http.Request) {
	keys, err := boltdb.GetAll[plexus.Key](keyTable)
	if err != nil {
		processError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := templates.ExecuteTemplate(w, keyTable, keys); err != nil {
		slog.Error("execute template", "template", keyTable, "keys", keys, "error", err)
	}
}

func deleteKey(w http.ResponseWriter, r *http.Request) {
	keyid := r.PathValue("id")
	key, err := boltdb.Get[plexus.Key](keyid, keyTable)
	if err != nil {
		processError(w, http.StatusBadRequest, "key does not exist")
		return
	}
	if err := removeKey(key); err != nil {
		processError(w, http.StatusInternalServerError, "delete key "+err.Error())
		return
	}
	displayKeys(w, r)
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
	config := Configuration{}
	if err := configuration.Get(&config); err != nil {
		slog.Error("configuration", "error", err)
		return "", err
	}
	device, err := nkeys.CreateUser()
	if err != nil {
		return "", err
	}
	seed, err := device.Seed()
	if err != nil {
		return "", err
	}
	keyValue := plexus.KeyValue{
		URL:     config.FQDN,
		Seed:    string(seed),
		KeyName: name,
	}
	payload, err := json.Marshal(&keyValue)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(payload), nil
}

func decrementKeyUsage(name string) error {
	key, err := boltdb.Get[plexus.Key](name, keyTable)
	if err != nil {
		return err
	}
	if key.Usage == 1 {
		return removeKey(key)
	}
	key.Usage--
	if err := boltdb.Save(key, key.Name, keyTable); err != nil {
		return err
	}
	return nil
}

func expireKeys() {
	slog.Debug("checking for expired keys")
	keys, err := boltdb.GetAll[plexus.Key](keyTable)
	if err != nil {
		slog.Error("get keys", "error", err)
	}
	for _, key := range keys {
		if key.Expires.Before(time.Now()) {
			slog.Info(
				"key has expired ...deleting",
				"key", key.Name,
				"expiry time", key.Expires.Format(time.RFC822),
			)
			if err := removeKey(key); err != nil {
				slog.Error("remove key", "error", err)
			}
		}
	}
}

func removeKey(key plexus.Key) error {
	var errs error
	if err := boltdb.Delete[plexus.Key](key.Name, keyTable); err != nil {
		slog.Error("delete key from db", "error", err)
		errs = errors.Join(errs, err)
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

func addDevice(token string) error {
	slog.Info("new login device", "device", token)
	keyValue, err := plexus.DecodeToken(token)
	if err != nil {
		slog.Error("decode token", "error", err)
		return err
	}
	key, err := nkeys.FromSeed([]byte(keyValue.Seed))
	if err != nil {
		slog.Error("seed failure", "error", err)
		return err
	}
	nPubKey, err := key.PublicKey()
	if err != nil {
		slog.Error("publickey", "error", err)
		return err
	}
	natsOptions.Nkeys = append(natsOptions.Nkeys, &server.NkeyUser{
		Nkey:        nPubKey,
		Permissions: registerPermissions(),
	})
	return natServer.ReloadOptions(natsOptions)
}
