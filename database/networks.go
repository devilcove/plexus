package database

import (
	"encoding/json"

	"github.com/devilcove/plexus"
	"go.etcd.io/bbolt"
)

func SaveNetwork(u *plexus.Network) error {
	value, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(NETWORKS_TABLE_NAME))
		return b.Put([]byte(u.Name), value)
	})
}

func GetNetwork(name string) (plexus.Network, error) {
	network := plexus.Network{}
	if err := db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket([]byte(NETWORKS_TABLE_NAME)).Get([]byte(name))
		if v == nil {
			return ErrNoResults
		}
		if err := json.Unmarshal(v, &network); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return network, err
	}
	return network, nil
}

func GetAllNetworks() ([]plexus.Network, error) {
	var networks []plexus.Network
	var network plexus.Network
	if err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(NETWORKS_TABLE_NAME))
		if b == nil {
			return ErrNoResults
		}
		_ = b.ForEach(func(k, v []byte) error {
			if err := json.Unmarshal(v, &network); err != nil {
				return err
			}
			networks = append(networks, network)
			return nil
		})
		return nil
	}); err != nil {
		return networks, err
	}
	return networks, nil
}

func DeleteNetwork(name string) error {
	if _, err := GetNetwork(name); err != nil {
		return err
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(NETWORKS_TABLE_NAME)).Delete([]byte(name))
	}); err != nil {
		return err
	}
	return nil
}
