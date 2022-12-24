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
)

func main() {
	flag.Parse()

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

	d := bytedribble.NewDownloader(meta, http.DefaultClient)
	err = d.SyncTracker(context.Background(), bytedribble.EmptyEvent)
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

	fmt.Println(peer.Initialize(context.Background()))
}
