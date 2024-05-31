package main

import (
	"fmt"
	"net"
	"os"
)

func startServing(addr net.Addr, contactAddr net.Addr) {
	fmt.Println("Starting server at", addr.String())

	if contactAddr != nil {
		fmt.Println("Contacting server at", contactAddr.String())
	}

	//TODO: implement the server logic
	/* from guidebook
	with SimpleXMLRPCServer((addr.ip, addr.port)) as server:
	        server.register_introspection_functions()
	        server.register_instance(RaftNode(KVStore(), addr, contact_node_addr))
	        server.serve_forever()
	*/
}

func main() {
	// take command line arguments (ip and port)
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run client.go <ip> <port>")
		fmt.Println("Usage: go run client.go <ip> <port> <contact_ip> <contact_port>")
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
