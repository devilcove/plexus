package database

import (
	"errors"
	"os"
	"time"

	"go.etcd.io/bbolt"
)

const (
	// table names
	USERS_TABLE_NAME    = "users"
	NETWORKS_TABLE_NAME = "networks"
	PEERS_TABLE_NAME    = "peers"
	KEYS_TABLE_NAME     = "keys"
	SETTINGS_TABLE_NAME = "settings"
	// errors
	NO_RECORDS = "no results found"
)

var (
	ErrNoResults = errors.New("no results found")
	db           *bbolt.DB
)

func InitializeDatabase() error {
	var err error
	file := os.Getenv("DB_FILE")
	if file == "" {
		file = "time.db"
	}
	db, err = bbolt.Open(file, 0666, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	return createTables()
}

func Close() {
	if err := db.Close(); err != nil {
		panic(err)
	}
}

func createTables() error {
	if err := createTable(USERS_TABLE_NAME); err != nil {
		return err
	}
	if err := createTable(NETWORKS_TABLE_NAME); err != nil {
		return err
	}
	if err := createTable(KEYS_TABLE_NAME); err != nil {
		return err
	}
	if err := createTable(PEERS_TABLE_NAME); err != nil {
		return err
	}
	if err := createTable(SETTINGS_TABLE_NAME); err != nil {
		return err
	}
	return nil
}

func createTable(name string) error {
	if err := db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
