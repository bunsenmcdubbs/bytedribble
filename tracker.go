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

type TorrentMetrics interface {
	Uploaded() int
	Downloaded() int
	Left() int
}

type FakeMetrics struct {
	TotalSize int
}

func (m FakeMetrics) Uploaded() int {
	return 0
}
func (m FakeMetrics) Downloaded() int {
	return 0
}
func (m FakeMetrics) Left() int {
	return m.TotalSize
}

type TrackerClient struct {
	client   *http.Client
	target   Metainfo
	selfInfo PeerInfo
	metrics  TorrentMetrics

	mu        sync.Mutex
	peerCache []PeerInfo
}

func NewTrackerClient(client *http.Client, target Metainfo, self PeerInfo, metrics TorrentMetrics) *TrackerClient {
	return &TrackerClient{
		client:   client,
		target:   target,
		selfInfo: self,
		metrics:  metrics,
	}
}

func (c *TrackerClient) Run(ctx context.Context) error {
	interval, err := c.syncTracker(ctx, Empty)
	if err != nil {
		return err
	}
	syncTrackerTicker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-syncTrackerTicker.C:
			newInterval, err := c.syncTracker(ctx, Empty)
			if err != nil {
				// TODO consider swallowing this error
				return err
			}
			if newInterval != interval {
				syncTrackerTicker.Reset(newInterval)
			}
		}
	}
}

type Event string

const (
	Started   Event = "started"
	Stopped   Event = "stopped"
	Completed Event = "completed"
	Empty     Event = ""
)

// syncTracker syncs with the torrent's tracker. Uploads metrics and current progress and receives a peer list.
//
// See: https://www.bittorrent.org/beps/bep_0003.html#trackers
// TODO implement UDP tracker support https://www.bittorrent.org/beps/bep_0015.html
func (c *TrackerClient) syncTracker(ctx context.Context, event Event) (time.Duration, error) {
	req, err := c.createTrackerRequest(ctx, event)
	if err != nil {
		return 0, fmt.Errorf("failed to create tracker request: %w", err)
	}
	interval, peers, err := c.sendTrackerRequest(req)
	if err != nil {
		return 0, err
	}

	c.mu.Lock()
	c.peerCache = peers
	c.mu.Unlock()

	return interval, nil
}

func (c *TrackerClient) createTrackerRequest(ctx context.Context, event Event) (*http.Request, error) {
	query := url.Values{}
	query.Set("info_hash", string(c.target.InfoHash()))
	query.Set("peer_id", string(c.selfInfo.PeerID[:]))
	// TODO ip? (optional)
	query.Set("port", strconv.Itoa(c.selfInfo.Port))
	query.Set("uploaded", strconv.Itoa(c.metrics.Uploaded()))
	query.Set("downloaded", strconv.Itoa(c.metrics.Downloaded()))
	query.Set("left", strconv.Itoa(c.metrics.Left()))
	// TODO implement support for parsing compact peer list https://www.bittorrent.org/beps/bep_0023.html
	query.Set("compact", "0") // Disable compact peer list
	if event != Empty {
		query.Set("event", string(event))
	}
	return http.NewRequestWithContext(ctx, http.MethodGet, c.target.TrackerURL.String()+"?"+query.Encode(), nil)
}

func (c *TrackerClient) sendTrackerRequest(req *http.Request) (time.Duration, []PeerInfo, error) {
	rawResp, err := c.client.Do(req)
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

func (c *TrackerClient) RequestNewPeers(ctx context.Context) ([]PeerInfo, error) {
	_, err := c.syncTracker(ctx, Empty)
	if err != nil {
		return nil, err
	}
	return c.Peers(), nil
}

func (c *TrackerClient) Peers() []PeerInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	peers := make([]PeerInfo, len(c.peerCache))
	copy(peers, c.peerCache)
	return peers
}

func (c *TrackerClient) Started(ctx context.Context) error {
	_, err := c.syncTracker(ctx, Started)
	return err
}

func (c *TrackerClient) Stopped(ctx context.Context) error {
	_, err := c.syncTracker(ctx, Stopped)
	return err
}

func (c *TrackerClient) Completed(ctx context.Context) error {
	_, err := c.syncTracker(ctx, Completed)
	return err
}
