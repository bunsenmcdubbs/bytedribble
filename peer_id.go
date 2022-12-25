package bytedribble

import (
	"bytes"
	"math/rand"
)

const peerIDLen = 20

type PeerID [peerIDLen]byte

func PeerIDFromString(s string) PeerID {
	p := *new(PeerID)
	copy(p[:], s)
	return p
}

func RandPeerID() PeerID {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := bytes.NewBuffer([]byte("abd0000-"))
	for i := b.Len(); i < peerIDLen; i++ {
		b.WriteByte(chars[rand.Intn(len(chars))])
	}
	return PeerIDFromString(b.String())
}

func (i PeerID) Bytes() []byte {
	return i[:]
}

func (i PeerID) String() string {
	return string(i[:])
}
