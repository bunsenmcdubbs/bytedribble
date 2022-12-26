package bytedribble

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBitfield_Has(t *testing.T) {
	type args struct {
		pieceIdx int
	}
	tests := []struct {
		name string
		b    Bitfield
		args args
		want bool
	}{
		{
			name: "true",
			b:    Bitfield{0x00, 0b10000000},
			args: args{8},
			want: true,
		},
		{
			name: "false",
			b:    Bitfield{0xff, 0b11111110},
			args: args{15},
			want: false,
		},
		{
			name: "nil case",
			b:    nil,
			args: args{12},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.b.Has(tt.args.pieceIdx), "Has(%v)", tt.args.pieceIdx)
		})
	}
}

func TestBitfield_Have(t *testing.T) {
	got := Bitfield{0x0e, 0xf0}
	got.Have(7)
	want := Bitfield{0x0f, 0xf0}
	assert.Equal(t, want, got)
}

func TestBitfield_Validate(t *testing.T) {
	type args struct {
		numPieces int
	}
	tests := []struct {
		name    string
		b       Bitfield
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "pass, exact",
			b:    Bitfield{0x00, 0xff, 0xf2},
			args: args{
				numPieces: 23,
			},
			wantErr: assert.NoError,
		},
		{
			name: "pass, empty",
			b:    Bitfield{0x00, 0xff, 0x00},
			args: args{
				numPieces: 21,
			},
			wantErr: assert.NoError,
		},
		{
			name: "too many bytes",
			b:    Bitfield{0x00, 0xff, 0x00, 0x00},
			args: args{
				numPieces: 23,
			},
			wantErr: assert.Error,
		},
		{
			name: "too many bits",
			b:    Bitfield{0x00, 0xff, 0x01},
			args: args{
				numPieces: 23,
			},
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, tt.b.Validate(tt.args.numPieces), fmt.Sprintf("Validate(%v)", tt.args.numPieces))
		})
	}
}
