package server

import (
	"testing"

	"github.com/Kairum-Labs/should"
	"github.com/devilcove/boltdb"
)

func Test_emailValid(t *testing.T) {
	tests := []struct {
		name string
		args string
		want bool
	}{
		{
			name: "valid",
			args: "someone@gmail.com",
			want: true,
		},
		{
			name: "invalid",
			args: "someone@",
			want: false,
		},
		{
			name: "example.com",
			args: "robbie@example.com",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.name, tt.args, tt.want)
			good := emailValid(tt.args)
			should.BeEqual(t, good, tt.want)
		})
	}
}

func TestConfigureServer(t *testing.T) {
	setup(t)
	defer shutdown(t)
	err := boltdb.Close()
	should.NotBeError(t, err)
	defer func() {
		err := boltdb.Initialize("./test.db",
			[]string{userTable, keyTable, networkTable, peerTable, settingTable})
		should.NotBeError(t, err)
	}()

	t.Run("secureNoFQDM", func(t *testing.T) {
		writeTmpConfg(t, &Configuration{Secure: true, FQDN: ""})
		config, err := configureServer()
		should.BeErrorIs(t, err, ErrSecureBlankFQDN)
		should.BeNil(t, config)
	})

	t.Run("secureWithIP", func(t *testing.T) {
		writeTmpConfg(t, &Configuration{Secure: true, FQDN: "10.10.10.100"})
		config, err := configureServer()
		should.BeErrorIs(t, err, ErrSecureWithIP)
		should.BeNil(t, config)
	})

	t.Run("secureNoEmail", func(t *testing.T) {
		writeTmpConfg(t, &Configuration{Secure: true, FQDN: "example.com"})
		config, err := configureServer()
		should.BeErrorIs(t, err, ErrInValidEmail)
		should.BeNil(t, config)
	})

	t.Run("insecure", func(t *testing.T) {
		writeTmpConfg(t, &Configuration{FQDN: "example.com", Email: "admin@domain.com"})
		config, err := configureServer()
		should.NotBeError(t, err)
		should.BeNil(t, config)
		t.Log(err, config)
		should.BeNil(t, boltdb.Close())
	})
}
