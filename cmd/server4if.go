// +build linux
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"

	"github.com/ugonakawaka/PacketSockDgram/packetsockdgram"
)

var (
	// AppName application name
	AppName = flag.String("name", "TEST UDP SERVER", "application name")
	// Port port
	Port = flag.Int("port", 55501, "message receive port")
	// Addr4 ip address version4
	Addr4 = flag.String("addr4", "127.0.0.1", "message receive address")
	// Addr6 ip address version6
	Addr6 = flag.String("addr6", "::1", "message receive address")
	// Dev target device
	Dev = flag.String("if", "lo", "interface")
	b   = flag.Bool("b", false, "payload trim right side LF")
)

func main() {
	flag.Parse()
	fmt.Println(*AppName, *Port, *Addr4, *Dev, *b)

	cnn, err := packetsockdgram.NewConnIf(*Port, *Dev)
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
