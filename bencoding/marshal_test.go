package bencoding

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMarshal(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want []byte
	}{
		{
			name: "empty string",
			in:   "",
			want: []byte("0:"),
		},
		{
			name: "simple string",
			in:   "hello",
			want: []byte("5:hello"),
		},
		{
			name: "positive number",
			in:   13084891,
			want: []byte("i13084891e"),
		},
		{
			name: "negative number",
			in:   -31,
			want: []byte("i-31e"),
		},
		{
			name: "zero",
			in:   0,
			want: []byte("i0e"),
		},
		{
			name: "int list",
			in:   []int{10, -34, 0, 3},
			want: []byte("li10ei-34ei0ei3ee"),
		},
		{
			name: "string list",
			in:   []string{"hello", "world", "beeeepboooop"},
			want: []byte("l5:hello5:world12:beeeepboooope"),
		},
		{
			name: "map[string]int",
			in: map[string]int{
				"hello": 123,
				"abc":   -444,
			},
			want: []byte("d3:abci-444e5:helloi123ee"),
		},
		{
			name: "map[string]string",
			in: map[string]string{
				"hello": "world",
				"abc":   "alphabet",
			},
			want: []byte("d3:abc8:alphabet5:hello5:worlde"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Marshal(tt.in))
		})
	}
}
