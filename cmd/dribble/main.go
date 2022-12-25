package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/bunsenmcdubbs/bytedribble"
	"log"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	metainfoPath := flag.Arg(0)
	if metainfoPath == "" {
		log.Fatalln("missing path to torrent file")
	}

	metainfoFile, err := os.Open(metainfoPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer metainfoFile.Close()

	meta, err := bytedribble.ParseMetainfo(metainfoFile)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Tracker URL:", meta.TrackerURL.String())
	fmt.Println("Infohash (hex):", hex.EncodeToString(meta.InfoHash()))
	fmt.Println("Piece size (bytes):", meta.PieceSizeBytes)

	d := bytedribble.NewDownloader(meta, http.DefaultClient)
	err = d.SyncTracker(ctx, bytedribble.EmptyEvent)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Client ID:", d.PeerID())
	peers := d.Peers()
	fmt.Println("Peers:", peers)

	var peer *bytedribble.Peer
	for _, info := range peers {
		log.Println("connecting to", info)
		peer, err = d.ConnectPeer(info.PeerID)
		if err != nil {
			log.Println("err:", err)
		}
		if peer != nil {
			break
		}
	}
	if peer == nil {
		log.Fatalln("unable to connect to any peer")
	}

	log.Println("Handshake complete! err:", peer.Initialize(context.Background()))

	go func() {
		log.Println("Expressed interest. err:", peer.StartDownload(ctx))
		log.Println("Peer unchoked us")

		log.Println("Download starting...")
		payload, err := peer.EnqueueRequest(ctx, bytedribble.RequestParams{
			PieceIndex:  0,
			BeginOffset: 0,
			Length:      16384,
		})
		log.Println("Request", err, payload)
	}()
	log.Println("Run complete. err:", peer.Run(ctx))

}
