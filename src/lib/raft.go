package lib

import (
	"net"
	"sync"
)

type NodeType int

const (
	LEADER NodeType = iota
	CANDIDATE
	FOLLOWER
)

// constants for RaftNode
const HEARTBEAT_INTERVAL = 1
const ELECTION_TIMEOUT_MIN = 2
const ELECTION_TIMEOUT_MAX = 3
const RPC_TIMEOUT = 0.5

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
	electionTerm    int
	clusterAddrList []net.Addr
	clusterLeader   net.Addr
}
