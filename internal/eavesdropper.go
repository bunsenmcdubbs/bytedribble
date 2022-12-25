package internal

import (
	"log"
	"net"
)

const defaultPrintLimit = 50

type Eavesdropper struct {
	net.Conn
	PrintLimit int
}

func (e Eavesdropper) Read(buf []byte) (n int, err error) {
	n, err = e.Conn.Read(buf)
	if e.PrintLimit != -1 || (e.PrintLimit == 0 && n > defaultPrintLimit) || (e.PrintLimit > 0 && n > e.PrintLimit) {
		log.Printf("Read raw bytes: %v [%d more bytes elided...]", buf[:e.PrintLimit], len(buf)-e.PrintLimit)
	} else {
		log.Printf("Read raw bytes: %v\n", buf)
	}
	return n, err
}

func (e Eavesdropper) Write(buf []byte) (n int, err error) {
	n, err = e.Conn.Write(buf)
	if e.PrintLimit != -1 || (e.PrintLimit == 0 && n > defaultPrintLimit) || (e.PrintLimit > 0 && n > e.PrintLimit) {
		log.Printf("Wrote raw bytes: %v [%d more bytes elided...]", buf[:e.PrintLimit], len(buf)-e.PrintLimit)
	} else {
		log.Printf("Wrote raw bytes: %v\n", buf)
	}
	return n, err
}
