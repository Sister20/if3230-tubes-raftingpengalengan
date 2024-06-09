package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"rafting/lib"
)

func startServing(addr net.Addr, contactAddr *net.Addr) {
	fmt.Println("Starting server at", addr.String())

	server := rpc.NewServer()
	node := lib.NewRaftNode(addr, contactAddr)
	err := server.Register(node)
	if err != nil {
		log.Fatalf("Error registering RaftNode: %v", err)
	}

	l, e := net.Listen("tcp", addr.String())
	if e != nil {
		log.Fatal("listen error:", e)
	}

	server.Accept(l)
}

func main() {
	// register to gob (all structs for safety)
	gob.Register(&net.TCPAddr{})
	gob.Register(&lib.AppendEntriesRequest{})
	gob.Register(&lib.AppendEntriesResponse{})
	gob.Register(&lib.LogEntry{})
	gob.Register(&lib.RaftVoteRequest{})
	gob.Register(&lib.RaftVoteResponse{})
	gob.Register(&[]lib.LogEntry{})

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
	contactNetAddr := net.Addr(contactAddr)

	startServing(addr, &contactNetAddr)
}
