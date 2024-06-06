package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"rafting/lib"
)

func startServing(addr net.Addr, contactAddr net.Addr) {
	fmt.Println("Starting server at", addr.String())

	if contactAddr != nil {
		fmt.Println("Contacting server at", contactAddr.String())
		// implement the logic to contact the other node if necessary
	}

	raftNode := lib.NewRaftNode(addr, &contactAddr)

	err := rpc.Register(raftNode)
	if err != nil {
		fmt.Println("Error registering RaftNode:", err)
		return
	}

	listener, err := net.Listen("tcp", addr.String())
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}

	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			fmt.Println("Error closing listener:", err)
		}
	}(listener)

	fmt.Println("Server is listening on", addr.String())

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}

func main() {
	// take command line arguments (ip and port)
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run server.go <ip> <port>")
		fmt.Println("Usage: go run server.go <ip> <port> [contact_ip] [contact_port]")
		os.Exit(0)
	}

	addr, err := net.ResolveTCPAddr("tcp", os.Args[1]+":"+os.Args[2])
	if err != nil {
		fmt.Println("Error resolving address")
		os.Exit(0)
	}

	if len(os.Args) == 3 {
		startServing(addr, nil)
		return
	}

	contactAddr, err := net.ResolveTCPAddr("tcp", os.Args[3]+":"+os.Args[4])
	if err != nil {
		fmt.Println("Error resolving contact address")
		os.Exit(0)
	}

	startServing(addr, contactAddr)
}
