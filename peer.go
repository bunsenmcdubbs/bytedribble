package bytedribble

import (
	"encoding/hex"
	"net"
	"sync"
)

const peerIDLen = 20

type PeerID [peerIDLen]byte

func PeerIDFromString(s string) PeerID {
	p := *new(PeerID)
	copy(p[:], s)
	return p
}

func (i PeerID) String() string {
	return hex.EncodeToString(i[:])
}

type PeerInfo struct {
	PeerID PeerID
	IP     net.IP
	Port   int
}

type Peer struct {
	info PeerInfo

	mu      sync.Mutex
	conn    net.Conn
	connErr error
}
