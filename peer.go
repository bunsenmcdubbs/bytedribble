package bytedribble

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"
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

type PeerInfo struct {
	PeerID PeerID
	IP     net.IP
	Port   int
}

type Peer struct {
	d *Downloader

	info       PeerInfo
	conn       net.Conn
	choked     bool
	interested bool
}

func NewPeer(info PeerInfo, d *Downloader) *Peer {
	return &Peer{
		d:          d,
		info:       info,
		choked:     true,
		interested: false,
	}
}

// Initialize establishes a connection to the peer and performs the initial handshake.
func (p *Peer) Initialize(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("handshake failed: %w", err)
		}
	}()

	dialer := net.Dialer{
		KeepAlive: 2 * time.Minute,
	}
	p.conn, err = dialer.DialContext(ctx, "tcp", (&net.TCPAddr{
		IP:   p.info.IP,
		Port: p.info.Port,
	}).String())

	if err != nil {
		return err
	}

	msg := append([]byte("\x13BitTorrent protocol\x00\x00\x00\x00\x00\x00\x00\x00"), p.d.Metainfo().InfoHash()...)
	_, err = p.conn.Write(msg)
	if err != nil {
		return err
	}

	resp := make([]byte, len(msg), len(msg))
	_, err = io.ReadFull(p.conn, resp)
	if err != nil {
		return err
	}
	if string(msg[:20]) != string(resp[:20]) {
		return errors.New("mismatched protocol")
	}
	if string(msg[28:]) != string(resp[28:]) {
		return errors.New("mismatched infohash")
	}

	_, err = p.conn.Write(p.d.PeerID().Bytes())
	if err != nil {
		return err
	}

	resp = make([]byte, peerIDLen, peerIDLen)
	_, err = io.ReadFull(p.conn, resp)
	if err != nil {
		return err
	}
	if p.info.PeerID.String() != string(resp) {
		return errors.New("mismatched peer id")
	}

	return nil
}
