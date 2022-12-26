package bytedribble

import (
	"errors"
	"math/bits"
)

type Bitfield []byte

func EmptyBitfield(numPieces int) Bitfield {
	return make(Bitfield, (numPieces+7)/8)
}

func (b Bitfield) Empty() bool {
	if len(b) == 0 {
		return false
	}
	for _, b1 := range b {
		if b1 != 0 {
			return false
		}
	}
	return true
}

func (b Bitfield) Validate(numPieces int) error {
	numExpectedBytes := (numPieces + 7) / 8
	if len(b) != numExpectedBytes {
		return errors.New("too many bytes")
	}
	rightPadZeros := (len(b) * 8) - numPieces
	if bits.TrailingZeros8(b[len(b)-1]) < rightPadZeros {
		return errors.New("insufficient trailing 0's")
	}
	return nil
}

func (b Bitfield) Have(pieceIdx int) {
	byteIdx := pieceIdx / 8
	bitIdx := pieceIdx % 8
	b[byteIdx] |= 0x80 >> bitIdx
}

func (b Bitfield) Has(pieceIdx int) bool {
	if len(b) == 0 {
		return false
	}
	byteIdx := pieceIdx / 8
	bitIdx := pieceIdx % 8
	return (b[byteIdx] & (0x80 >> bitIdx)) > 0
}
