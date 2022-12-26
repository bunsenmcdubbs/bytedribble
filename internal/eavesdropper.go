package internal

import (
	"log"
	"net"
)

const defaultPrintLimit = 50

type Eavesdropper struct {
	net.Conn
	limit int
}

func NewEavesdropper(conn net.Conn) *Eavesdropper {
	return &Eavesdropper{
		Conn:  conn,
		limit: defaultPrintLimit,
	}
}

func (e Eavesdropper) Read(buf []byte) (n int, err error) {
	n, err = e.Conn.Read(buf)
	if e.limit != -1 && n > e.limit {
		log.Printf("Read raw bytes: %v [%d more bytes elided...]", buf[:e.limit], n-e.limit)
	} else {
		log.Printf("Read raw bytes: %v\n", buf)
	}
	return n, err
}

func (e Eavesdropper) Write(buf []byte) (n int, err error) {
	n, err = e.Conn.Write(buf)
	if e.limit != -1 && n > e.limit {
		log.Printf("Wrote raw bytes: %v [%d more bytes elided...]", buf[:e.limit], n-e.limit)
	} else {
		log.Printf("Wrote raw bytes: %v\n", buf)
	}
	return n, err
}
