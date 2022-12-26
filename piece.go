package bytedribble

import (
	"crypto/sha1"
	"fmt"
)

type Piece struct {
	Index     uint32
	Size      uint32
	BlockSize uint32
	Hash      [sha1.Size]byte

	blocks  [][]byte // blocks are views into payload
	payload []byte
}

func (p *Piece) NumBlocks() int {
	return int((p.Size + p.BlockSize - 1) / p.BlockSize)
}

func (p *Piece) block(idx int) Block {
	if idx < 0 || idx >= p.NumBlocks() {
		panic("invalid block index")
	}

	length := p.BlockSize
	if idx == p.NumBlocks()-1 && p.Size%p.BlockSize != 0 {
		length = p.Size % p.BlockSize
	}
	return Block{
		PieceIndex:  p.Index,
		BeginOffset: uint32(idx) * p.BlockSize,
		Length:      length,
	}
}

func (p *Piece) MissingBlocks() []Block {
	if p.blocks == nil {
		p.blocks = make([][]byte, p.NumBlocks(), p.NumBlocks())
	}
	var missing []Block
	for idx, block := range p.blocks {
		if block == nil {
			missing = append(missing, p.block(idx))
		}
	}
	return missing
}

func (p *Piece) AddBlockPayload(block Block, payload []byte) {
	idx := int(block.BeginOffset / p.BlockSize)
	if block != p.block(idx) {
		panic("alien block")
	}
	if len(payload) != int(block.Length) {
		panic("mismatched block length")
	}
	if p.blocks == nil {
		p.blocks = make([][]byte, p.NumBlocks(), p.NumBlocks())
	}
	if p.payload == nil {
		p.payload = make([]byte, p.Size, p.Size)
	}

	copy(p.payload[block.BeginOffset:block.BeginOffset+block.Length], payload)
	p.blocks[idx] = p.payload[block.BeginOffset : block.BeginOffset+block.Length : block.BeginOffset+block.Length]
}

func (p *Piece) Valid() bool {
	if len(p.MissingBlocks()) != 0 {
		return false
	}
	return sha1.Sum(p.payload) == p.Hash
}

func (p *Piece) Payload() []byte {
	return p.payload
}

func (p *Piece) String() string {
	return fmt.Sprintf("{Index: %d; Size: %d; Hash: %v}", p.Index, p.Size, p.Hash)
}

func (p *Piece) Reset() {
	p.blocks = nil
	// keep p.payload around to avoid reallocating memory
}
