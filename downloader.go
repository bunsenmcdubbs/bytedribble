package bytedribble

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"sync"
)

type Downloader struct {
	tc     *TrackerClient
	target Metainfo
	self   PeerInfo
}

func NewDownloader(target Metainfo, self PeerInfo) *Downloader {
	d := &Downloader{
		tc:     NewTrackerClient(http.DefaultClient, target, self, FakeMetrics{TotalSize: target.TotalSizeBytes}),
		target: target,
		self:   self,
	}
	return d
}

func (d *Downloader) Start(ctx context.Context) {
	go func() {
		for {
			err := d.tc.Run(ctx)
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Println("Sync tracker error:", err)
		}
	}()

	peers, err := d.tc.RequestNewPeers(ctx) // TODO better peer list API
	if err != nil {
		log.Println("Failed to get new peers")
		// TODO better error handling
	}

	// TODO refactor this into a more legible, resilient, and correct form
	var pieceMu sync.Mutex
	pending := make(map[uint32]*Piece)
	inProgress := make(map[uint32]*Piece)
	complete := make(map[uint32]*Piece)
	for idx, hash := range d.target.Hashes {
		size := d.target.TotalSizeBytes % d.target.PieceSizeBytes
		if idx != len(d.target.Hashes)-1 {
			size = d.target.PieceSizeBytes
		}
		pending[uint32(idx)] = &Piece{
			Index:     uint32(idx),
			Size:      uint32(size),
			BlockSize: DefaultBlockLength,
			Hash:      hash,
		}
		log.Println("Pending piece", pending[uint32(idx)])
	}

	startNextPiece := func() *Piece {
		pieceMu.Lock()
		defer pieceMu.Unlock()
		for _, next := range pending {
			delete(pending, next.Index)
			inProgress[next.Index] = next
			return next
		}
		return nil
	}
	var doneOnce sync.Once
	doneC := make(chan struct{})
	completePiece := func(p *Piece) {
		pieceMu.Lock()
		defer pieceMu.Unlock()
		delete(inProgress, p.Index)
		complete[p.Index] = p
		// TODO announce HAVE piece to all peers
		log.Println("Finished piece", p)
		if len(pending) == 0 && len(inProgress) == 0 {
			log.Println("Done with all pieces")
			doneOnce.Do(func() {
				close(doneC)
			})
		}
	}
	failPiece := func(p *Piece) {
		pieceMu.Lock()
		defer pieceMu.Unlock()
		delete(inProgress, p.Index)
		// TODO maybe this doesn't work?
		p.Reset()
		pending[p.Index] = p
	}

	workersGroup, workersCtx := errgroup.WithContext(ctx)
	workersGroup.SetLimit(2) // TODO configure this limit

	var workersMu sync.Mutex
	workers := make(map[PeerID]*Worker)

	for _, info := range peers {
		log.Println("Attempting to connect to", info)
		if info.PeerID == d.self.PeerID {
			continue
		}
		func(info PeerInfo) {
			workersGroup.Go(func() (err error) {
				defer func() {
					if err != nil {
						log.Printf("peer %s disconnected: %v", info.PeerID, err)
						err = nil
						workersMu.Lock()
						delete(workers, info.PeerID)
						workersMu.Unlock()
					}
				}()

				workersMu.Lock()
				if _, exists := workers[info.PeerID]; exists {
					// TODO eat this error
					return errors.New("already connected")
				}
				peer := NewPeer(info, d.self.PeerID, d.target.InfoHash(), len(d.target.Hashes))
				worker := NewWorker(peer)
				worker.SetCallback(func(piece *Piece, err error) {
					if err != nil {
						log.Println("failed download:", err)
						failPiece(piece)
					} else {
						completePiece(piece)
					}
					next := startNextPiece()
					if next != nil {
						worker.RequestPiece(next)
					}
				})
				workers[info.PeerID] = worker
				workersMu.Unlock()

				if err = peer.Initialize(workersCtx); err != nil {
					return fmt.Errorf("unable to initialize connection: %w", err)
				}

				go worker.Run(ctx)
				next := startNextPiece()
				if next != nil {
					worker.RequestPiece(next)
				}
				<-doneC
				return nil
			})
		}(info)
	}

	log.Println("Workers finished! Error:", workersGroup.Wait())
	log.Println("Notified tracker. Error: ", d.tc.Completed(ctx))
}
