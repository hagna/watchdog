package main

import (
	"flag"
	"fmt"
	"net"
)

var msg = flag.String("m", "bar", "message to send to the server")
var ep = flag.String("ep", "0.0.0.0:3212", "network endpoint")

func main() {
	flag.Parse()
	raddr, err := net.ResolveUDPAddr("udp", *ep)
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		fmt.Println("error dialing", err)
		return
	}
	defer conn.Close()
	b := []byte(*msg)
	n, err := conn.Write(b)
	if err != nil {
		fmt.Println("error writing", err)
		return
	}
	fmt.Printf("wrote %d bytes\n", n)
}
