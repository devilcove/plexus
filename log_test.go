package plexus_test

import (
	"testing"

	"github.com/devilcove/plexus"
)

func TestSetLogging(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
	}{
		{
			name: "debug",
		},
		{
			name: "info",
		},
		{
			name: "warn",
		},
		{
			name: "error",
		},
		{
			name: "junk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			plexus.SetLogging(tt.name)
		})
	}
}
