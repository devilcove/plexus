package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			assert.Equal(t, tt.want, good)
		})
	}
}
