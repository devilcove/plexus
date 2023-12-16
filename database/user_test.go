package database

import (
	"errors"
	"testing"
	"time"

	"github.com/devilcove/plexus"
	"github.com/stretchr/testify/assert"
)

func TestSaveUser(t *testing.T) {
	user := plexus.User{
		Username: "tester",
		Password: "testpass",
		IsAdmin:  false,
		Updated:  time.Now(),
	}
	err := SaveUser(&user)
	assert.Nil(t, err)
}

func TestGetUser(t *testing.T) {
	err := deleteAllUsers(false)
	assert.Nil(t, err)
	t.Run("admin", func(t *testing.T) {
		user, err := GetUser("admin")
		assert.Nil(t, err)
		assert.Equal(t, "admin", user.Username)
		assert.Equal(t, true, user.IsAdmin)
	})
	t.Run("user", func(t *testing.T) {
		err := createTestUser(plexus.User{
			Username: "testing",
			Password: "testing",
		})
		assert.Nil(t, err)
		user, err := GetUser("testing")
		assert.Nil(t, err)
		assert.Equal(t, "testing", user.Username)
		assert.Equal(t, false, user.IsAdmin)
	})
	t.Run("nouser", func(t *testing.T) {
		user, err := GetUser("test2")
		assert.Equal(t, plexus.User{}, user)
		assert.NotNil(t, err)
		assert.Equal(t, "no results found", err.Error())
	})
}

func TestGetUsers(t *testing.T) {
	err := deleteAllUsers(true)
	assert.Nil(t, err)
	t.Run("nousers", func(t *testing.T) {
		users, err := GetAllUsers()
		assert.Nil(t, err)
		assert.Equal(t, []plexus.User(nil), users)
	})
	t.Run("multiple", func(t *testing.T) {
		err = createTestUser(plexus.User{
			Username: "testing",
			Password: "testing",
		})
		assert.Nil(t, err)
		err = createTestUser(plexus.User{
			Username: "testing2",
			Password: "testing",
		})
		assert.Nil(t, err)
		users, err := GetAllUsers()
		assert.Nil(t, err)
		assert.Equal(t, 2, len(users))
	})
}

func TestDeleteUser(t *testing.T) {
	err := deleteAllUsers(true)
	assert.Nil(t, err)
	t.Run("nosuchuser", func(t *testing.T) {
		err := DeleteUser("admin")
		assert.NotNil(t, err)
		assert.Equal(t, "no results found", err.Error())
	})
	t.Run("existinguser", func(t *testing.T) {
		err := createTestUser(plexus.User{
			Username: "testing",
			Password: "testing",
		})
		assert.Nil(t, err)
		err = DeleteUser("testing")
		assert.Nil(t, err)
	})
}

func TestAdminExists(t *testing.T) {
	err := deleteAllUsers(true)
	assert.Nil(t, err)
	t.Run("noadmin", func(t *testing.T) {
		admin := AdminExist()
		assert.False(t, admin)
	})
	t.Run("adminexists", func(t *testing.T) {
		err := createTestUser(plexus.User{
			Username: "admin",
			Password: "testing",
			IsAdmin:  true,
		})
		assert.Nil(t, err)
		admin := AdminExist()
		assert.True(t, admin)
	})
}

func createTestUser(user plexus.User) (err error) {
	user.Password, err = HashPassword(user.Password)
	if err != nil {
		return err
	}
	if err := SaveUser(&user); err != nil {
		return err
	}
	return nil
}

func deleteAllUsers(deleteAll bool) (errs error) {
	users, err := GetAllUsers()
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Username != "admin" || deleteAll == true {
			if err := DeleteUser(user.Username); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	return errs
}
