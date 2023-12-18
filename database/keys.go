package database

import (
	"encoding/json"

	"github.com/devilcove/plexus"
	"go.etcd.io/bbolt"
)

func SaveKey(u *plexus.Key) error {
	value, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(KEYS_TABLE_NAME))
		return b.Put([]byte(u.ID), value)
	})
}

func GetKey(ID string) (plexus.Key, error) {
	key := plexus.Key{}
	if err := db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte(KEYS_TABLE_NAME)).Get([]byte(ID))
		if v == nil {
			return ErrNoResults
		}
		if err := json.Unmarshal(v, &key); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return key, err
	}
	return key, nil
}

func GetAllKeys() ([]plexus.Key, error) {
	var keys []plexus.Key
	var key plexus.Key
	if err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(KEYS_TABLE_NAME))
		if b == nil {
			return ErrNoResults
		}
		_ = b.ForEach(func(k, v []byte) error {
			if err := json.Unmarshal(v, &key); err != nil {
				return err
			}
			keys = append(keys, key)
			return nil
		})
		return nil
	}); err != nil {
		return keys, err
	}
	return keys, nil
}

func DeleteKey(ID string) error {
	if _, err := GetKey(ID); err != nil {
		return err
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(KEYS_TABLE_NAME)).Delete([]byte(ID))
	}); err != nil {
		return err
	}
	return nil
}
