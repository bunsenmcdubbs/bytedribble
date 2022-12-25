package bytedribble

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/bunsenmcdubbs/bytedribble/internal"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
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

	reqMu    sync.Mutex
	inFlight map[RequestParams]requestCallback
	requestQ chan RequestParams
	cancelQ  chan RequestParams

	info PeerInfo
	conn net.Conn

	// Local's interest in remote peer
	interestedMu sync.Mutex
	interested   bool

	// Remote's choke/unchoke status on local peer
	choked     bool
	unchokedCh chan struct{} // closed if and only if channel is unchoked
}

func NewPeer(info PeerInfo, d *Downloader) *Peer {
	return &Peer{
		d:          d,
		inFlight:   make(map[RequestParams]requestCallback),
		requestQ:   make(chan RequestParams),
		cancelQ:    make(chan RequestParams),
		info:       info,
		interested: false,
		choked:     true,
		unchokedCh: make(chan struct{}, 0),
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

	p.conn = internal.Eavesdropper{Conn: p.conn}

	if err != nil {
		return err
	}

	msg := append([]byte(defaultHeader), p.d.Metainfo().InfoHash()...)
	_, err = p.conn.Write(msg)
	if err != nil {
		return err
	}

	resp := make([]byte, len(msg), len(msg))
	_, err = io.ReadFull(p.conn, resp)
	if err != nil {
		return err
	}
	if err = validateHeader(msg); err != nil {
		return err
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

const defaultHeader = "\x13BitTorrent protocol\x00\x00\x00\x00\x00\x00\x00\x00"

func validateHeader(header []byte) error {
	if len(header) < 28 {
		return errors.New("invalid header length")
	}

	if string(header[:20]) != defaultHeader[:20] {
		return errors.New("invalid protocol")
	}
	// ignore extension options for now
	return nil
}

type MessageType byte

const (
	Choke MessageType = iota
	Unchoke
	Interested
	NotInterested
	Have
	Bitfield
	Request
	Piece
	Cancel
)

func (p *Peer) Run(ctx context.Context) error {
	childCtx, cancelCtx := context.WithCancel(ctx)
	defer cancelCtx()
	go func() { p.keepAlive(childCtx) }()
	go func() {
		<-ctx.Done()
		_ = p.conn.SetDeadline(time.Now())
	}()
	defer p.conn.Close()
	for {
		if ctx.Err() != nil {
			return nil
		}

		log.Println("Waiting for next message from remote")
		header := make([]byte, 5)
		nBytes, err := io.ReadFull(p.conn, header)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue
		} else if err != nil {
			return fmt.Errorf("unable to read message: %w", err)
		}

		log.Printf("Received raw bytes: %v\n", header)
		if nBytes == 5 {
			err := p.handleMessage(binary.BigEndian.Uint32(header[:4]), MessageType(header[4]))
			if err != nil {
				return err
			}
		}
	}
}

func (p *Peer) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("sending keepalive")
			_, err := p.conn.Write([]byte{0})
			if err != nil {
				log.Println("failed to send keepalive:", err)
			}
		}
	}
}

func (p *Peer) handleMessage(msgLen uint32, typ MessageType) error {
	log.Println("Received message. Length:", msgLen)
	switch typ {
	case Choke:
		fmt.Println("Choke")
		if !p.choked {
			p.unchokedCh = make(chan struct{}, 0)
		}
	case Unchoke:
		fmt.Println("Unchoke")
		close(p.unchokedCh)
	case Interested:
		fmt.Println("Interested")
	case NotInterested:
		fmt.Println("Not interested")
	case Have:
		fmt.Println("Have")
	case Bitfield:
		fmt.Println("Bitfield")
		length := msgLen - 1
		bitfield := make([]byte, length, length)
		_, err := io.ReadFull(p.conn, bitfield)
		if err != nil {
			return err
		}
		fmt.Println(hex.Dump(bitfield))
	case Request:
		fmt.Println("Request")
	case Piece:
		fmt.Println("Piece")
		length := msgLen - 9
		header := make([]byte, 8, 8)
		_, err := io.ReadFull(p.conn, header)
		if err != nil {
			return err
		}
		pieceIdx := binary.BigEndian.Uint32(header[:4])
		beginOffset := binary.BigEndian.Uint32(header[4:])
		piecePayload := make([]byte, length, length)
		_, err = io.ReadFull(p.conn, piecePayload)
		if err != nil {
			piecePayload = nil
		}
		req := RequestParams{
			PieceIndex:  pieceIdx,
			BeginOffset: beginOffset,
			Length:      length,
		}
		p.reqMu.Lock()
		callback, exists := p.inFlight[req]
		if exists {
			callback(piecePayload, err)
			delete(p.inFlight, req)
		}
		p.reqMu.Unlock()
		if err != nil {
			return err
		}
	case Cancel:
		fmt.Println("Cancel")
	default:
		return fmt.Errorf("unrecognized message type: %d", typ)
	}
	return nil
}

func (p *Peer) Choke() error {
	log.Println("Sending Choke")
	_, err := p.conn.Write([]byte{0, 0, 0, 1, byte(Choke)})
	return err
}
func (p *Peer) Unchoke() error {
	log.Println("Sending Unchoke")
	_, err := p.conn.Write([]byte{0, 0, 0, 1, byte(Unchoke)})
	return err
}

func (p *Peer) Interested() error {
	log.Println("Sending Interested")
	p.interestedMu.Lock()
	defer p.interestedMu.Unlock()
	if p.interested {
		return nil
	}
	if n, err := p.conn.Write([]byte{0, 0, 0, 1, byte(Interested)}); err != nil {
		return err
	} else {
		fmt.Printf("wrote %d bytes\n", n)
	}
	p.interested = true
	return nil
}

func (p *Peer) NotInterested() error {
	log.Println("Sending NotInterested")
	p.interestedMu.Lock()
	defer p.interestedMu.Unlock()
	if !p.interested {
		return nil
	}
	if _, err := p.conn.Write([]byte{0, 0, 0, 1, byte(NotInterested)}); err != nil {
		return err
	}
	p.interested = false
	return nil
}

func (p *Peer) Have(pieceIdx uint32) error {
	log.Printf("Sending Have. Piece %d", pieceIdx)
	bs := []byte{0, 0, 0, 5, byte(Have)}
	binary.BigEndian.AppendUint32(bs, pieceIdx)
	_, err := p.conn.Write(bs)
	return err
}

type RequestParams struct {
	PieceIndex  uint32
	BeginOffset uint32
	Length      uint32
}

func (r RequestParams) requestMessage() []byte {
	b := make([]byte, 0, 17)
	b = binary.BigEndian.AppendUint32(b, 13)
	b = append(b, byte(Request))
	b = binary.BigEndian.AppendUint32(b, r.PieceIndex)
	b = binary.BigEndian.AppendUint32(b, r.BeginOffset)
	b = binary.BigEndian.AppendUint32(b, r.Length)
	return b
}

func (r RequestParams) cancelMessage() []byte {
	b := make([]byte, 0, 17)
	b = binary.BigEndian.AppendUint32(b, 13)
	b = append(b, byte(Cancel))
	b = binary.BigEndian.AppendUint32(b, r.PieceIndex)
	b = binary.BigEndian.AppendUint32(b, r.BeginOffset)
	b = binary.BigEndian.AppendUint32(b, r.Length)
	return b
}

func (p *Peer) Request(params RequestParams) error {
	log.Println("Sending Request")
	_, err := p.conn.Write(params.requestMessage())
	return err
}

func (p *Peer) Cancel(param RequestParams) error {
	log.Println("Sending Cancel")
	_, err := p.conn.Write(param.cancelMessage())
	return err
}

type requestCallback func(piece []byte, err error)

func (p *Peer) Unchoked() <-chan struct{} {
	return p.unchokedCh
}

func (p *Peer) StartDownload(ctx context.Context) error {
	if err := p.Interested(); err != nil {
		return err
	}
	select {
	case <-p.Unchoked():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Peer) EnqueueRequest(ctx context.Context, req RequestParams) ([]byte, error) {
	// TODO remove ctx?
	var piece []byte
	errCh := make(chan error, 1)
	defer close(errCh)

	p.reqMu.Lock()
	if _, exists := p.inFlight[req]; exists {
		p.reqMu.Unlock()
		return nil, errors.New("request already in flight")
	}
	p.inFlight[req] = func(pieceResp []byte, errResp error) {
		piece = pieceResp
		errCh <- errResp
	}
	p.reqMu.Unlock()

	if err := p.Request(req); err != nil {
		_ = p.CancelRequest(req)
		return nil, err
	}

	log.Println("Sent request. Waiting for response...")
	var err error
	select {
	case <-ctx.Done():
	case err = <-errCh:
	}

	return piece, err
}

func (p *Peer) CancelRequest(req RequestParams) error {
	var exists bool
	p.reqMu.Lock()
	_, exists = p.inFlight[req]
	delete(p.inFlight, req)
	p.reqMu.Unlock()
	if !exists {
		return nil
	}
	if err := p.Cancel(req); err != nil {
		return err
	}
	return nil
}
