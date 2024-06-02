package main

import (
	"fmt"
	"net"
	"os"
)

func startClient(addr net.Addr, contactAddr net.Addr) {
	fmt.Println("Starting client at", addr.String())

	if contactAddr != nil {
		fmt.Println("Contacting server at", contactAddr.String())
	}

	// TODO: implement the client logic
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run client.go <ip> <port>")
		fmt.Println("Usage: go run client.go <ip> <port> [contact_ip] [contact_port]")
		os.Exit(0)
	}

	addr, err := net.ResolveTCPAddr("tcp", os.Args[1]+":"+os.Args[2])
	if err != nil {
		fmt.Println("Error resolving address")
		os.Exit(0)
	}

	if len(os.Args) == 3 {
		startClient(addr, nil)
		return
	}

	contactAddr, err := net.ResolveTCPAddr("tcp", os.Args[3]+":"+os.Args[4])
	if err != nil {
		fmt.Println("Error resolving contact address")
		os.Exit(0)
	}

	startClient(addr, contactAddr)
}
