package bytedribble

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/bunsenmcdubbs/bytedribble/internal"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type PeerInfo struct {
	PeerID PeerID
	IP     net.IP
	Port   int
}

type Peer struct {
	self     PeerID
	infohash []byte
	info     PeerInfo
	conn     net.Conn
	stopOnce sync.Once
	stopC    chan struct{}

	subscriber chan<- Message

	peerHas Bitfield // peer's Bitfield

	// Local's interest in remote peer
	interestedMu sync.Mutex
	interested   bool

	unchokedCh chan struct{} // closed if and only if peer has unchoked us
}

func NewPeer(info PeerInfo, self PeerID, infohash []byte, numPieces int) *Peer {
	return &Peer{
		self:       self,
		infohash:   infohash,
		info:       info,
		stopC:      make(chan struct{}),
		peerHas:    EmptyBitfield(numPieces),
		unchokedCh: make(chan struct{}),
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

	p.conn = internal.NewEavesdropper(p.conn)

	if err != nil {
		return err
	}

	msg := append([]byte(defaultHeader), p.infohash...)
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

	_, err = p.conn.Write(p.self.Bytes())
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

type Message struct {
	Type    MessageType
	Payload []byte
}

type MessageType byte

const (
	ChokeMessage MessageType = iota
	UnchokeMessage
	InterestedMessage
	NotInterestedMessage
	HaveMessage
	BitfieldMessage
	RequestMessage
	PieceMessage
	CancelMessage
)

func (p *Peer) Run() error {
	go func() { p.keepAliveLoop() }()
	go func() {
		<-p.stopC
		_ = p.conn.SetDeadline(time.Now())
	}()
	defer p.conn.Close()
	defer p.Close()

	for {
		select {
		case <-p.stopC:
			return nil
		default:
		}

		log.Println("Waiting for next message from remote")
		header := make([]byte, 5)
		_, err := io.ReadFull(p.conn, header)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return fmt.Errorf("unable to read message: %w", err)
		}

		messageType := MessageType(header[4])

		payload := make([]byte, binary.BigEndian.Uint32(header[:4])-1, binary.BigEndian.Uint32(header[:4])-1)
		_, err = io.ReadFull(p.conn, payload)
		if err != nil {
			return fmt.Errorf("unable to read message: %w", err)
		}

		// TODO handle interested/uninterested
		switch messageType {
		case ChokeMessage:
			select {
			case <-p.unchokedCh:
				p.unchokedCh = make(chan struct{})
			default:
			}
		case UnchokeMessage:
			select {
			case <-p.unchokedCh:
			default:
				close(p.unchokedCh)
			}
		case HaveMessage:
			p.peerHas.Have(int(binary.BigEndian.Uint32(payload)))
		case BitfieldMessage:
			if p.peerHas.Empty() {
				p.peerHas = payload // TODO validate
			}
		}

		if p.subscriber != nil {
			select {
			case p.subscriber <- Message{
				Type:    messageType,
				Payload: payload,
			}:
			default:
			}
		}
	}
}

func (p *Peer) keepAliveLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopC:
			return
		case <-ticker.C:
			log.Println("sending keepalive")
			if err := p.KeepAlive(); err != nil {
				log.Println("failed to send keepalive:", err)
			}
		}
	}
}

func (p *Peer) KeepAlive() error {
	_, err := p.conn.Write([]byte{0})
	return err
}

func (p *Peer) Close() {
	p.stopOnce.Do(func() {
		close(p.stopC)
	})
}

func (p *Peer) Choke() error {
	log.Println("Sending Choke")
	_, err := p.conn.Write([]byte{0, 0, 0, 1, byte(ChokeMessage)})
	return err
}
func (p *Peer) Unchoke() error {
	log.Println("Sending Unchoke")
	_, err := p.conn.Write([]byte{0, 0, 0, 1, byte(UnchokeMessage)})
	return err
}

func (p *Peer) Interested() error {
	log.Println("Sending Interested")
	p.interestedMu.Lock()
	defer p.interestedMu.Unlock()
	if p.interested {
		return nil
	}
	if _, err := p.conn.Write([]byte{0, 0, 0, 1, byte(InterestedMessage)}); err != nil {
		return err
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
	if _, err := p.conn.Write([]byte{0, 0, 0, 1, byte(NotInterestedMessage)}); err != nil {
		return err
	}
	p.interested = false
	return nil
}

func (p *Peer) Have(pieceIdx uint32) error {
	log.Printf("Sending Have. Piece %d", pieceIdx)
	bs := []byte{0, 0, 0, 5, byte(HaveMessage)}
	binary.BigEndian.AppendUint32(bs, pieceIdx)
	_, err := p.conn.Write(bs)
	return err
}

func (p *Peer) Request(params Block) error {
	log.Println("Sending Request")
	_, err := p.conn.Write(params.requestMessage())
	return err
}

func (p *Peer) Cancel(param Block) error {
	log.Println("Sending Cancel")
	_, err := p.conn.Write(param.cancelMessage())
	return err
}

func (p *Peer) Info() PeerInfo {
	return p.info
}

func (p *Peer) Unchoked() <-chan struct{} {
	return p.unchokedCh
}

func (p *Peer) Subscribe(messageCh chan<- Message) error {
	if p.subscriber != nil && p.subscriber != messageCh {
		return errors.New("a different subscriber is already listening")
	}
	p.subscriber = messageCh
	return nil
}

func (p *Peer) Unsubscribe(messageCh chan<- Message) error {
	// TODO check if this (in)equality actually works
	if p.subscriber != messageCh {
		return errors.New("not currently subscribed")
	}
	p.subscriber = nil
	return nil
}
