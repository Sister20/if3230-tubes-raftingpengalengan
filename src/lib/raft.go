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
	HeartbeatInterval  = 1 * time.Second
	ElectionTimeoutMin = 2 * time.Second
	ElectionTimeoutMax = 3 * time.Second
	RpcTimeout         = 500 * time.Millisecond
)

type LogEntry struct {
	Term    int
	Command string
}

// TODO: Adjust the struct fields as needed
type RaftNode struct {
	mu              sync.Mutex
	address         net.Addr
	nodeType        NodeType
	app             *KVStore
	clusterAddrList []net.Addr
	clusterLeader   *net.TCPAddr
	contactAddr     *net.Addr
	heartbeatTicker *time.Ticker
	electionTerm    int
	electionTimeout *time.Ticker

	// Persistent server states
	currentTerm int
	votedFor    net.Addr
	log         []LogEntry

	// Volatile server states
	commitIndex int
	lastApplied int

	// Volatile leader states
	nextIndex  map[net.Addr]int
	matchIndex map[net.Addr]int
}

type RaftVoteRequest struct {
	Term         int
	CandidateId  net.Addr
	LastLogIndex int
	LastLogTerm  int
}

type RaftVoteResponse struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesRequest struct {
	Term         int
	LeaderId     net.Addr
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesResponse struct {
	Term    int
	Success bool
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
		heartbeatTicker: time.NewTicker(HeartbeatInterval),
		electionTimeout: time.NewTicker(time.Duration(ElectionTimeoutMin.Nanoseconds()+rand.Int63n(ElectionTimeoutMax.Nanoseconds()-ElectionTimeoutMin.Nanoseconds())) * time.Nanosecond),
		currentTerm:     0,
		votedFor:        nil,
		commitIndex:     0,
		lastApplied:     0,
		nextIndex:       make(map[net.Addr]int),
		matchIndex:      make(map[net.Addr]int),
	}

	if contactAddr == nil {
		node.initializeAsLeader()
	} else {
		node.tryToApplyMembership(*contactAddr)
	}

	// UNCOMMENT INI KALAU MAU CEK LEADER AMA CLUSTER ADDRESS LISTNYA DIA
	//go func() {
	//	ticker := time.NewTicker(1 * time.Second)
	//	for range ticker.C {
	//		node.mu.Lock() // Lock to prevent data race
	//		fmt.Println("Cluster Leader:", node.clusterLeader)
	//		fmt.Println("Cluster Address List:", node.clusterAddrList)
	//		node.mu.Unlock() // Unlock after reading
	//	}
	//}()

	return node
}

func (node *RaftNode) initializeAsLeader() {
	node.mu.Lock()
	defer node.mu.Unlock()
	log.Println("Initializing as leader node...")
	node.nodeType = LEADER
	tcpAddr, ok := node.address.(*net.TCPAddr)
	if !ok {
		log.Printf("Error converting address to TCP address")
	}
	node.clusterLeader = tcpAddr
	node.clusterAddrList = append(node.clusterAddrList, node.address)
	go node.leaderHeartbeat()
}

func (node *RaftNode) leaderHeartbeat() {
	for range node.heartbeatTicker.C {
		log.Println("[Leader] Sending heartbeat...")

		request := &AppendEntriesRequest{
			Term: -99,
		}

		for _, addr := range node.clusterAddrList {
			if addr.String() == node.address.String() {
				continue
			}

			go node.sendRequest("RaftNode.AppendEntries", addr, request)
		}
	}
}

func (node *RaftNode) RequestVote(args *RaftVoteRequest) RaftVoteResponse {
	node.mu.Lock()
	defer node.mu.Unlock()

	// Return false if the candidate's term is less than the current term
	if args.Term > node.log[len(node.log)-1].Term {
		return RaftVoteResponse{node.currentTerm, false}
	}

	if (node.votedFor == nil || node.votedFor.String() == args.CandidateId.String()) && (args.LastLogIndex >= len(node.log)-1 && args.LastLogTerm >= node.log[len(node.log)-1].Term) {
		node.votedFor = args.CandidateId
		return RaftVoteResponse{node.currentTerm, true}
	} else {
		return RaftVoteResponse{node.currentTerm, false}
	}
}

func (node *RaftNode) AppendEntries(args *AppendEntriesRequest, reply *[]byte) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	if args.Term == -99 {
		log.Println("Received heartbeat...")

		// Reset election timeout
		node.electionTimeout.Stop()
		node.electionTimeout = time.NewTicker(time.Duration(ElectionTimeoutMin.Nanoseconds()+rand.Int63n(ElectionTimeoutMax.Nanoseconds()-ElectionTimeoutMin.Nanoseconds())) * time.Nanosecond)

		responseMap := map[string]interface{}{
			"term":    node.currentTerm,
			"success": true,
		}
		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Printf("Error marshalling response: %v", err)
		}
		*reply = responseBytes

		return nil
	}

	//if args.Term < node.currentTerm {
	//	log.Println("Rejecting AppendEntries... (Term is less than current term)")
	//	*reply = AppendEntriesResponse{node.currentTerm, false}
	//	return nil
	//}
	//
	//if len(node.log)-1 < args.PrevLogIndex {
	//	log.Println("Rejecting AppendEntries... (Log is shorter)")
	//	*reply = AppendEntriesResponse{node.currentTerm, false}
	//	return nil
	//} else if node.log[args.PrevLogIndex].Term != args.PrevLogTerm {
	//	log.Println("Rejecting AppendEntries... (Term mismatch)")
	//	*reply = AppendEntriesResponse{node.currentTerm, false}
	//	return nil
	//}
	//
	//node.log = node.log[:args.PrevLogIndex+1]
	//node.log = append(node.log, args.Entries...)
	//
	//if args.LeaderCommit > node.commitIndex {
	//	node.commitIndex = min(args.LeaderCommit, len(node.log)-1)
	//}
	//
	//log.Println("Appending entries successfully...")
	//*reply = AppendEntriesResponse{node.currentTerm, true}
	//return nil

	// TODO: HAPUS INI, HANYA PLACEHOLDER
	*reply = []byte("Placeholder")
	return nil
}

func (node *RaftNode) startElection() {
	node.mu.Lock()

	log.Println("Starting election...")
	node.nodeType = CANDIDATE
	node.electionTerm++
	node.clusterLeader = nil

	node.mu.Unlock()

	voteCount := 1

	for _, addr := range node.clusterAddrList {
		if addr.String() == node.address.String() {
			continue
		}

		go func(addr net.Addr) {
			response := node.sendRequest("RaftNode.RequestVote", addr, RaftVoteRequest{
				Term:         node.electionTerm,
				CandidateId:  node.address,
				LastLogIndex: len(node.log) - 1,
				LastLogTerm:  0,
			})

			var result RaftVoteResponse
			err := json.Unmarshal(response, &result)
			if err != nil {
				log.Printf("Error unmarshalling response: %v", err)
			}

			if result.Term > node.electionTerm {
				node.mu.Lock()
				defer node.mu.Unlock()
				node.nodeType = FOLLOWER
				node.electionTerm = result.Term
				return
			}

			if result.VoteGranted {
				voteCount++
				if voteCount > len(node.clusterAddrList)/2+1 {
					node.mu.Lock()
					defer node.mu.Unlock()
					node.nodeType = LEADER
					tcpAddr, ok := node.address.(*net.TCPAddr)
					if !ok {
						log.Printf("Error converting address to TCP address")
					}
					node.clusterLeader = tcpAddr
					go node.leaderHeartbeat()
				}
			}
		}(addr)
	}
}

func (node *RaftNode) tryToApplyMembership(contactAddr net.Addr) {
	for {
		response := node.sendRequest("RaftNode.ApplyMembership", contactAddr, node.address)
		var result map[string]interface{}

		err := json.Unmarshal(response, &result)
		if err != nil {
			log.Printf("Error unmarshalling response: %v", err)
		}

		status := result["status"].(string)
		if status == "success" {
			node.mu.Lock()

			node.clusterAddrList = parseAddresses(result["clusterAddrList"].([]interface{}))
			temp := parseAddress(result["clusterLeader"].(string))
			tcpAddr, ok := temp.(*net.TCPAddr)
			if !ok {
				log.Println("Error converting address to TCP address")
			}
			node.clusterLeader = tcpAddr

			node.mu.Unlock()
			break
		} else if status == "redirected" {
			newAddr := parseAddress(result["address"].(string))
			contactAddr = newAddr
		}
	}
}

func (node *RaftNode) ApplyMembership(args *net.TCPAddr, reply *[]byte) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	node.clusterAddrList = append(node.clusterAddrList, args)

	clusterAddrList := make([]string, len(node.clusterAddrList))
	for i, addr := range node.clusterAddrList {
		clusterAddrList[i] = addr.String()
	}

	clusterLeaderStr := node.clusterLeader.String()

	responseMap := map[string]interface{}{
		"status":          "success",
		"clusterAddrList": clusterAddrList,
		"clusterLeader":   clusterLeaderStr,
	}

	responseBytes, err := json.Marshal(responseMap)
	if err != nil {
		log.Printf("Error marshalling response: %v", err)
	}
	*reply = responseBytes

	return nil
}

func (node *RaftNode) sendRequest(method string, addr net.Addr, request interface{}) []byte {
	conn, err := net.DialTimeout("tcp", addr.String(), RpcTimeout)
	if err != nil {
		log.Printf("Dialing failed: %v", err)
	}

	if conn == nil {
		log.Printf("Error dialing to address: %v\n", addr)
		return nil
	}

	client := rpc.NewClient(conn)
	defer func(client *rpc.Client) {
		if client != nil {
			err := client.Close()
			if err != nil {
				log.Fatalf("Error closing client: %v\n", err)
			}
		}
	}(client)

	var response []byte
	err = client.Call(method, request, &response)
	if err != nil {
		log.Printf("RPC failed: %v", err)
	}
	return response
}

func parseAddress(addr string) net.Addr {
	address, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Printf("Error resolving address: %v", err)
	}
	return address
}

func parseAddresses(data []interface{}) []net.Addr {
	addresses := make([]net.Addr, 0)
	for _, addr := range data {
		address, err := net.ResolveTCPAddr("tcp", addr.(string))
		if err != nil {
			log.Printf("Error resolving address (%v): %v", addr, err)
		}
		addresses = append(addresses, address)
	}
	return addresses
}
