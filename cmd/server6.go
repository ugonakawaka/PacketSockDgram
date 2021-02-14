// +build linux
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"syscall"

	"github.com/ugonakawaka/PacketSockDgram/packetsockdgram"
)

var (
	// AppName application name
	AppName = flag.String("name", "TEST UDP SERVER", "application name")
	// Addr6 ip address version6
	Addr6 = flag.String("addr6", "::1", "message receive address")
	// Port port
	Port = flag.Int("port", 55501, "message receive port")
	// Dev target device
	Dev = flag.String("if", "lo", "interface")
	b   = flag.Bool("b", false, "payload trim right side LF")
)

func main() {
	flag.Parse()
	fmt.Println(*AppName, *Port, *Addr6, *Dev, *b)
	ip := net.ParseIP(*Addr6)
	b16 := [16]byte{}
	copy(b16[:], ip)
	sa := &syscall.SockaddrInet6{
		Addr: b16,
		Port: *Port,
	}
	cnn, err := packetsockdgram.NewConn(sa, *Dev)
	if err != nil {
		log.Fatalln(err)
	}

	defer cnn.Close()

	for {
		b := make([]byte, 1500)
		n, iph, udph, payload, err := cnn.Readfrom(b)

		fmt.Printf("err:[%v]\n", err)
		fmt.Printf("size:[%d]\n", n)
		fmt.Printf("%v\n", iph)
		fmt.Printf("%v\n", udph)
		fmt.Printf("payload:[%v]\n", string(payload))
		fmt.Printf("%s\n", hex.Dump(b[:n]))
		fmt.Println("")
	}
}
