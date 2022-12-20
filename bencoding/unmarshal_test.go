package bencoding

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name    string
		raw     []byte
		want    any
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "positive number",
			raw:     []byte("i234e"),
			want:    234,
			wantErr: assert.NoError,
		},
		{
			name:    "negative number",
			raw:     []byte("i-10e"),
			want:    -10,
			wantErr: assert.NoError,
		},
		//{
		//	name:    "invalid number (0 padding)",
		//	raw:     []byte("i010e"),
		//	wantErr: assert.Error,
		//},
		{
			name:    "invalid number (invalid contents)",
			raw:     []byte("i3f23e"),
			wantErr: assert.Error,
		},
		{
			name:    "invalid number (missing i)",
			raw:     []byte("33e"),
			wantErr: assert.Error,
		},
		{
			name:    "invalid number (missing e)",
			raw:     []byte("i33"),
			wantErr: assert.Error,
		},
		{
			name:    "string",
			raw:     []byte("22:hello, world! 123 i1el"),
			want:    "hello, world! 123 i1el",
			wantErr: assert.NoError,
		},
		{
			name:    "invalid string (length mismatch)",
			raw:     []byte("18:hello"),
			wantErr: assert.Error,
		},
		{
			name:    "[]int",
			raw:     []byte("li1ei2ei-10ee"),
			want:    []any{1, 2, -10},
			wantErr: assert.NoError,
		},
		{
			name:    "[]string",
			raw:     []byte("l7:hello, 6:world!e"),
			want:    []any{"hello, ", "world!"},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unmarshal(bytes.NewReader(tt.raw))
			if !tt.wantErr(t, err, fmt.Sprintf("Unmarshal(%s)", tt.raw)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Unmarshal(%s)", tt.raw)
		})
	}
}
