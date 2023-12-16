package main

import (
	"os"
	"testing"

	"github.com/devilcove/plexus/database"
	"github.com/stretchr/testify/assert"
)

func TestDefaultUser(t *testing.T) {
	t.Run("noadmim", func(t *testing.T) {
		err := deleteAllUsers(true)
		assert.Nil(t, err)
		checkDefaultUser()
		user, err := database.GetUser("admin")
		assert.Nil(t, err)
		assert.Equal(t, "admin", user.Username)
		assert.Equal(t, true, user.IsAdmin)
	})
	t.Run("env", func(t *testing.T) {
		err := deleteAllUsers(true)
		assert.Nil(t, err)
		err = os.Setenv("PLEXUS_USER", "Administrator")
		assert.Nil(t, err)
		checkDefaultUser()
		user, err := database.GetUser("Administrator")
		assert.Nil(t, err)
		assert.Equal(t, "Administrator", user.Username)
		assert.Equal(t, true, user.IsAdmin)
	})
	t.Run("adminexists", func(t *testing.T) {
		checkDefaultUser()
		user, err := database.GetUser("Administrator")
		assert.Nil(t, err)
		assert.Equal(t, "Administrator", user.Username)
		assert.Equal(t, true, user.IsAdmin)
	})
}
