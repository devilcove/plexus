package plexus

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Kairum-Labs/should"
)

func TestDecodeToken(t *testing.T) {
	t.Run("badEncoding", func(t *testing.T) {
		value, err := DecodeToken("bad")
		var expectedErr base64.CorruptInputError
		should.BeEqual(t, true, errors.As(err, &expectedErr))
		should.BeEqual(t, value, KeyValue{})
	})
	t.Run("bad", func(t *testing.T) {
		value, err := DecodeToken(base64.StdEncoding.EncodeToString([]byte("bad")))
		should.ContainSubstring(t, err.Error(), "invalid character")
		should.BeEqual(t, value, KeyValue{})
	})
	t.Run("good", func(t *testing.T) {
		value := KeyValue{
			URL:     "example.com",
			Seed:    "seed",
			KeyName: "testing",
		}
		payload, err := json.Marshal(&value)
		should.NotBeError(t, err)
		keyValue, err := DecodeToken(base64.StdEncoding.EncodeToString(payload))
		should.NotBeError(t, err)
		should.BeEqual(t, keyValue, value)
	})
}
