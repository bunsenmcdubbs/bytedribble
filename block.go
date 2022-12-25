package bytedribble

import "encoding/binary"

type Block struct {
	PieceIndex  uint32
	BeginOffset uint32
	Length      uint32
}

func (b Block) requestMessage() []byte {
	bs := make([]byte, 0, 17)
	bs = binary.BigEndian.AppendUint32(bs, 13)
	bs = append(bs, byte(RequestMessage))
	bs = binary.BigEndian.AppendUint32(bs, b.PieceIndex)
	bs = binary.BigEndian.AppendUint32(bs, b.BeginOffset)
	bs = binary.BigEndian.AppendUint32(bs, b.Length)
	return bs
}

func (b Block) cancelMessage() []byte {
	bs := make([]byte, 0, 17)
	bs = binary.BigEndian.AppendUint32(bs, 13)
	bs = append(bs, byte(CancelMessage))
	bs = binary.BigEndian.AppendUint32(bs, b.PieceIndex)
	bs = binary.BigEndian.AppendUint32(bs, b.BeginOffset)
	bs = binary.BigEndian.AppendUint32(bs, b.Length)
	return bs
}
