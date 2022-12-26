package bytedribble

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"sync"
)

type Worker struct {
	peer     *Peer
	callback func(*Piece, error)

	sendNextRequest chan struct{}

	mu         sync.Mutex
	inProgress map[uint32]*Piece
}

func NewWorker(peer *Peer) *Worker {
	return &Worker{
		peer:            peer,
		sendNextRequest: make(chan struct{}, 1),
		inProgress:      make(map[uint32]*Piece),
	}
}

func (w *Worker) SetCallback(cb func(*Piece, error)) {
	w.callback = cb
}

func (w *Worker) Run(ctx context.Context) error {
	messageCh := make(chan Message, 10)
	if err := w.peer.Subscribe(messageCh); err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- w.peer.Run()
		close(errCh)
		_ = w.peer.Unsubscribe(messageCh)
		close(messageCh)
	}()
	defer w.peer.Close()

	go w.requesterLoop(ctx)

	for {
		select {
		case err := <-errCh:
			return err
		case msg := <-messageCh:
			switch msg.Type {
			case PieceMessage:
				block := Block{
					PieceIndex:  binary.BigEndian.Uint32(msg.Payload[:4]),
					BeginOffset: binary.BigEndian.Uint32(msg.Payload[4:8]),
					Length:      uint32(len(msg.Payload) - 8),
				}
				log.Println("Received a block from peer", block)
				w.receiveBlock(block, msg.Payload[8:])
				select {
				case w.sendNextRequest <- struct{}{}:
				default:
				}
			}
		}
	}
}

func (w *Worker) RequestPiece(p *Piece) {
	log.Println("Worker requesting next piece", p.String())
	w.mu.Lock()
	w.inProgress[p.Index] = p
	w.mu.Unlock()
	// TODO fix bug with request queue. Worker is sending duplicate requests to peer
	select {
	case w.sendNextRequest <- struct{}{}:
	default:
	}
}

func (w *Worker) requesterLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.sendNextRequest:
		}

		w.mu.Lock()
		var block Block
		var foundBlock bool
		for _, p := range w.inProgress {
			if missing := p.MissingBlocks(); len(missing) > 0 {
				block = missing[0]
				foundBlock = true
				break
			}
		}
		w.mu.Unlock()
		if foundBlock {
			err := RetryWithExpBackoff(ctx, func(ctx context.Context) error {
				return w.requestBlock(ctx, block)
			}, 1, 5)
			if err != nil {
				w.mu.Lock()
				w.callback(w.inProgress[block.PieceIndex], fmt.Errorf("failed to request more blocks from piece: %w", err))
				w.mu.Unlock()
			}
		}
	}
}

func (w *Worker) requestBlock(ctx context.Context, b Block) error {
	log.Println("Requesting block", b)
	if err := w.peer.Interested(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.peer.Unchoked():
	}

	if err := w.peer.Request(b); err != nil {
		return fmt.Errorf("unable to send request: %w", err)
	}
	return nil
}

func (w *Worker) receiveBlock(block Block, payload []byte) {
	w.mu.Lock()
	piece, exists := w.inProgress[block.PieceIndex]
	w.mu.Unlock()
	if !exists {
		log.Println("received block for a piece that wasn't requested", block.PieceIndex)
		return
	}
	piece.AddBlockPayload(block, payload)
	if len(piece.MissingBlocks()) == 0 {
		w.mu.Lock()
		delete(w.inProgress, block.PieceIndex)
		w.mu.Unlock()
		if piece.Valid() {
			w.callback(piece, nil)
		} else {
			w.callback(piece, errors.New("hash mismatch"))
		}
	}
}
