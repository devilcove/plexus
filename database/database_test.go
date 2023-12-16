package database

import (
	"os"
	"testing"

	"github.com/devilcove/plexus"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	//main.setLogging()
	os.Setenv("DB_FILE", "test.db")
	_ = InitializeDatabase()
	defer Close()
	_ = createTestUser(plexus.User{
		Username: "admin",
		Password: "testing",
		IsAdmin:  true,
	})
	os.Exit(m.Run())
}

func TestCloseDB(t *testing.T) {
	t.Run("open", func(t *testing.T) {
		Close()
	})
	t.Run("closed", func(t *testing.T) {
		Close()
		err := InitializeDatabase()
		assert.Nil(t, err)
	})
}
