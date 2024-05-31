package lib

import (
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
)

type NodeType int

const (
	LEADER NodeType = iota
	CANDIDATE
	FOLLOWER
)

const (
	HEARTBEAT_INTERVAL   = 1 * time.Second
	ELECTION_TIMEOUT_MIN = 2 * time.Second
	ELECTION_TIMEOUT_MAX = 3 * time.Second
	RPC_TIMEOUT          = 500 * time.Millisecond
)

type LogEntry struct {
	Term    string
	Command string
}

type RaftNode struct {
	mu              sync.Mutex
	address         net.Addr
	nodeType        NodeType
	log             []LogEntry
	app             *KVStore
	clusterAddrList []net.Addr
	clusterLeader   *net.Addr
	contactAddr     *net.Addr
	heartbeatTicker *time.Ticker
	electionTerm    int
	electionTimeout *time.Ticker
}

func NewRaftNode(addr net.Addr, contactAddr *net.Addr) *RaftNode {
	node := &RaftNode{
		address:         addr,
		nodeType:        FOLLOWER,
		log:             make([]LogEntry, 0),
		app:             NewKVStore(),
		electionTerm:    0,
		clusterAddrList: make([]net.Addr, 0),
		clusterLeader:   nil,
		contactAddr:     contactAddr,
		heartbeatTicker: time.NewTicker(HEARTBEAT_INTERVAL),
		electionTimeout: time.NewTicker(time.Duration(ELECTION_TIMEOUT_MIN.Nanoseconds()+rand.Int63n(ELECTION_TIMEOUT_MAX.Nanoseconds()-ELECTION_TIMEOUT_MIN.Nanoseconds())) * time.Nanosecond),
	}

	if contactAddr == nil {
		node.initializeAsLeader()
	} else {
		node.tryToApplyMembership(*contactAddr)
	}

	return node
}

func (node *RaftNode) initializeAsLeader() {
	node.mu.Lock()
	defer node.mu.Unlock()
	log.Println("Initializing as leader node...")
	node.nodeType = LEADER
	node.clusterLeader = &node.address
	node.clusterAddrList = append(node.clusterAddrList, node.address)
	go node.leaderHeartbeat()
}

func (node *RaftNode) leaderHeartbeat() {
	for range node.heartbeatTicker.C {
		log.Println("[Leader] Sending heartbeat...")
		// TODO: implement sending heartbeat to all nodes in the cluster
	}
}

func (node *RaftNode) tryToApplyMembership(contactAddr net.Addr) {
	for {
		response := node.sendRequest("RaftNode.ApplyMembership", contactAddr, node.address)
		var result map[string]interface{}
		json.Unmarshal(response, &result)
		status := result["status"].(string)
		if status == "success" {
			node.mu.Lock()
			defer node.mu.Unlock()
			node.clusterAddrList = parseAddresses(result["clusterAddrList"].([]interface{}))
			temp := parseAddress(result["clusterLeader"].(map[string]interface{}))
			node.clusterLeader = &temp
			break
		} else if status == "redirected" {
			newAddr := parseAddress(result["address"].(map[string]interface{}))
			contactAddr = newAddr
		}
	}
}

func (node *RaftNode) sendRequest(method string, addr net.Addr, request interface{}) []byte {
	conn, err := net.DialTimeout("tcp", addr.String(), RPC_TIMEOUT)
	if err != nil {
		log.Fatalf("Dialing failed: %v", err)
	}
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Fatalf("Error closing connection: %v", err)
		}
	}(conn)

	client := rpc.NewClient(conn)
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			log.Fatalf("Error closing client: %v", err)
		}
	}(client)

	var response []byte
	err = client.Call(method, request, &response)
	if err != nil {
		log.Fatalf("RPC failed: %v", err)
	}
	return response
}

func parseAddress(addr map[string]interface{}) net.Addr {
	address, err := net.ResolveTCPAddr("tcp", addr["ip"].(string)+":"+addr["port"].(string))
	if err != nil {
		log.Fatalf("Error resolving address: %v", err)
	}
	return address
}

func parseAddresses(data []interface{}) []net.Addr {
	addresses := make([]net.Addr, 0)
	for _, addr := range data {
		address, err := net.ResolveTCPAddr("tcp", addr.(string))
		if err != nil {
			log.Fatalf("Error resolving address (%v): %v", addr, err)
		}
		addresses = append(addresses, address)
	}
	return addresses
}
