package bytedribble

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/bunsenmcdubbs/bytedribble/bencoding"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type Downloader struct {
	client *http.Client

	meta   Metainfo
	peerID PeerID

	// TODO add self IP and port
	// TODO add metrics uploaded, downloaded, left

	mu        sync.Mutex
	interval  time.Duration
	newPeers  map[PeerID]PeerInfo
	peerConns map[PeerID]*Peer
}

func NewDownloader(target Metainfo, client *http.Client) *Downloader {
	d := &Downloader{
		client:    client,
		meta:      target,
		peerID:    RandPeerID(),
		peerConns: make(map[PeerID]*Peer),
	}
	return d
}

type Event string

const (
	StartedEvent   = "started"
	StoppedEvent   = "stopped"
	CompletedEvent = "completed"
	EmptyEvent     = ""
)

// SyncTracker syncs Downloader with the tracker. Uploads metrics and current progress and receives a peer list.
//
// See: https://www.bittorrent.org/beps/bep_0003.html#trackers
// TODO implement UDP tracker support https://www.bittorrent.org/beps/bep_0015.html
func (d *Downloader) SyncTracker(ctx context.Context, event Event) error {
	req, err := d.createTrackerRequest(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to create tracker request: %w", err)
	}
	interval, peers, err := d.sendTrackerRequest(req)
	if err != nil {
		return err
	}

	newPeers := make(map[PeerID]PeerInfo)
	d.mu.Lock()
	d.interval = interval
	for _, peer := range peers {
		if _, connected := d.peerConns[peer.PeerID]; connected {
			continue
		}
		newPeers[peer.PeerID] = peer
	}
	d.newPeers = newPeers
	d.mu.Unlock()

	return nil
}

func (d *Downloader) createTrackerRequest(ctx context.Context, event Event) (*http.Request, error) {
	query := url.Values{}
	query.Set("info_hash", string(d.meta.InfoHash()))
	query.Set("peer_id", string(d.peerID[:]))
	// TODO ip? (optional)
	query.Set("port", "6881")                              // TODO fake port
	query.Set("uploaded", "0")                             // TODO fake "uploaded"
	query.Set("downloaded", "0")                           // TODO fake "downloaded"
	query.Set("left", strconv.Itoa(d.meta.TotalSizeBytes)) // TODO fake "left"
	query.Set("compact", "0")                              // Disable compact peer list
	// TODO implement support for parsing compact peer list https://www.bittorrent.org/beps/bep_0023.html
	if event != EmptyEvent {
		query.Set("event", string(event))
	}
	return http.NewRequestWithContext(ctx, http.MethodGet, d.meta.TrackerURL.String()+"?"+query.Encode(), nil)
}

func (d *Downloader) sendTrackerRequest(req *http.Request) (time.Duration, []PeerInfo, error) {
	rawResp, err := d.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	if rawResp.StatusCode != http.StatusOK {
		return 0, nil, fmt.Errorf("tracker responded with unexpected HTTP error code: %d", rawResp.StatusCode)
	}

	defer rawResp.Body.Close()
	resp, err := bencoding.UnmarshalDict(bufio.NewReader(rawResp.Body))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to parse tracker response: %w", err)
	}

	if failure, ok := resp["failure"]; ok {
		return 0, nil, fmt.Errorf("tracker returned error: %s", failure.(string))
	}

	intervalSeconds, ok := resp["interval"].(int)
	if !ok {
		return 0, nil, errors.New("missing interval")
	}

	var peers []PeerInfo
	peerDicts, ok := resp["peers"].([]any)
	if !ok {
		return 0, nil, errors.New("missing peer list")
	}
	for _, pd := range peerDicts {
		p := pd.(map[string]any)
		pi := PeerInfo{}
		if id, ok := p["peer id"].(string); !ok || len([]byte(id)) != peerIDLen {
			return 0, nil, errors.New("missing valid peer id")
		} else {
			pi.PeerID = PeerIDFromString(id)
		}
		if ipString, ok := p["ip"].(string); !ok {
			return 0, nil, errors.New("missing peer ip address")
		} else {
			pi.IP = net.ParseIP(ipString)
		}
		if port, ok := p["port"].(int); !ok {
			return 0, nil, errors.New("missing peer port number")
		} else {
			pi.Port = port
		}
		peers = append(peers, pi)
	}

	return time.Duration(intervalSeconds) * time.Second, peers, nil
}

func (d *Downloader) Peers() []PeerInfo {
	d.mu.Lock()
	defer d.mu.Unlock()
	var peers []PeerInfo
	for _, peer := range d.newPeers {
		peers = append(peers, peer)
	}
	for _, conn := range d.peerConns {
		peers = append(peers, conn.info)
	}
	return peers
}

func (d *Downloader) Metainfo() Metainfo {
	return d.meta
}

func (d *Downloader) PeerID() PeerID {
	return d.peerID
}

func (d *Downloader) ConnectPeer(id PeerID) (*Peer, error) {
	if id == d.peerID {
		return nil, errors.New("cannot connect to self")
	}
	d.mu.Lock()
	if _, connected := d.peerConns[id]; connected {
		d.mu.Unlock()
		return nil, errors.New("already connected to peer")
	}
	info, known := d.newPeers[id]
	d.mu.Unlock()
	if !known {
		return nil, errors.New("unknown peer id")
	}

	p := NewPeer(info, d)
	return p, nil
}
