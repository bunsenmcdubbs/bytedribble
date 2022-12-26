package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/bunsenmcdubbs/bytedribble"
	"log"
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

	d := bytedribble.NewDownloader(meta, bytedribble.PeerInfo{
		PeerID: bytedribble.PeerIDFromString("01234567890123456789"),
		IP:     nil,
		Port:   9424,
	})
	d.Start(ctx)
}
