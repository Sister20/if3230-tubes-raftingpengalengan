package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
)

type KVStore struct {
	store map[string]string
	mu    sync.Mutex
}

func NewKVStore() *KVStore {
	return &KVStore{store: make(map[string]string)}
}

func (kv *KVStore) Get(key string, value *string) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if v, ok := kv.store[key]; ok {
		*value = v
		return nil
	}
	return fmt.Errorf("key not found")
}

func (kv *KVStore) Put(kvPair [2]string, ack *bool) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store[kvPair[0]] = kvPair[1]
	*ack = true
	return nil
}

type RaftNode struct {
	store        *KVStore
	addr         net.Addr
	contactAddr  net.Addr
}

func NewRaftNode(store *KVStore, addr net.Addr, contactAddr net.Addr) *RaftNode {
	return &RaftNode{store: store, addr: addr, contactAddr: contactAddr}
}

// You can add RaftNode methods here to handle RPC calls.

func startServing(addr net.Addr, contactAddr net.Addr) {
	fmt.Println("Starting server at", addr.String())

	if contactAddr != nil {
		fmt.Println("Contacting server at", contactAddr.String())
		// Here you can implement the logic to contact the other node if necessary
	}

	kvStore := NewKVStore()
	raftNode := NewRaftNode(kvStore, addr, contactAddr)

	rpc.Register(raftNode)
	listener, err := net.Listen("tcp", addr.String())
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

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
