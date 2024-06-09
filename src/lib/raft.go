package lib

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"
	"strings"
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
	ElectionTimeoutMin = 6 * time.Second
	ElectionTimeoutMax = 9 * time.Second
	RpcTimeout         = 6 * time.Second
)

type LogEntry struct {
	Term    int
	Command string
}

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

// type ExecuteCommand

func NewRaftNode(addr net.Addr, contactAddr *net.Addr) *RaftNode {
	node := &RaftNode{
		address:         addr,
		nodeType:        FOLLOWER,
		log:             make([]LogEntry, 0),
		app:             NewKVStore(),
		electionTerm:    1,
		clusterAddrList: make([]net.Addr, 0),
		clusterLeader:   nil,
		contactAddr:     contactAddr,
		heartbeatTicker: time.NewTicker(HeartbeatInterval),
		electionTimeout: time.NewTicker(time.Duration(ElectionTimeoutMin.Nanoseconds()+rand.Int63n(ElectionTimeoutMax.Nanoseconds()-ElectionTimeoutMin.Nanoseconds())) * time.Nanosecond),
		currentTerm:     1,
		votedFor:        nil,
		commitIndex:     -1,
		lastApplied:     -1,
		nextIndex:       make(map[net.Addr]int),
		matchIndex:      make(map[net.Addr]int),
	}

	if contactAddr == nil {
		node.initializeAsLeader()
	} else {
		node.tryToApplyMembership(*contactAddr)
		// TODO: Fix timeout, it should be reset after receiving AppendEntries
		go func() {
			for {
				select {
				case <-node.electionTimeout.C:
					node.startElection()
					node.electionTimeout.Reset(time.Duration(ElectionTimeoutMin.Nanoseconds()+rand.Int63n(ElectionTimeoutMax.Nanoseconds()-ElectionTimeoutMin.Nanoseconds())) * time.Nanosecond)
				default:
					break
				}
			}
		}()
	}

	// UNCOMMENT INI KALAU MAU CEK LEADER AMA CLUSTER ADDRESS LISTNYA DIA
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			node.mu.Lock() // Lock to prevent data race
			fmt.Println("Cluster Leader:", node.clusterLeader)
			fmt.Println("Cluster Address List:", node.clusterAddrList)
			fmt.Println("Log entries:", node.log)
			if (node.contactAddr) != nil {
				fmt.Println("Contact address:", (*node.contactAddr).String())
			} else {
				fmt.Println("Contact address: nil")
			}
			node.mu.Unlock() // Unlock after reading
		}
	}()

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

	// Initialize nextIndex and matchIndex
	for _, addr := range node.clusterAddrList {
		node.nextIndex[addr] = len(node.log)
		node.matchIndex[addr] = -1
	}
	go node.leaderHeartbeat()
}

func (node *RaftNode) leaderHeartbeat() {
	for range node.heartbeatTicker.C {
		log.Println("[Leader] Sending heartbeat...")

		// Send AppendEntries to all nodes in the cluster
		responses := make(chan AppendEntriesResponse, len(node.clusterAddrList))
		var wg sync.WaitGroup

		for _, addr := range node.clusterAddrList {
			if addr.String() == node.address.String() {
				continue
			}

			// var prevLogTerm int
			// var entries []LogEntry
			// node.mu.Lock()
			// if node.nextIndex[addr]-1 < 0 && len(node.log) == 0 {
			// 	prevLogTerm = 0
			// 	entries = node.log[node.nextIndex[addr]:]
			// } else if node.nextIndex[addr]-1 < 0 && len(node.log) != 0 {
			// 	prevLogTerm = 0
			// 	entries = node.log[node.nextIndex[addr] : node.nextIndex[addr]+1]
			// } else if node.nextIndex[addr] == len(node.log) {
			// 	prevLogTerm = node.log[node.nextIndex[addr]-1].Term
			// 	entries = node.log[node.nextIndex[addr]:]
			// } else {
			// 	prevLogTerm = node.log[node.nextIndex[addr]-1].Term
			// 	entries = node.log[node.nextIndex[addr] : node.nextIndex[addr]+1]
			// }
			node.mu.Lock()

			var prevLogTerm int
			if node.nextIndex[addr]-1 < 0 {
				prevLogTerm = 0 // 0 means empty log
			} else {
				prevLogTerm = node.log[node.nextIndex[addr]-1].Term
			}

			request := &AppendEntriesRequest{
				Term:         node.currentTerm,
				LeaderId:     node.address,
				PrevLogIndex: node.nextIndex[addr] - 1,
				PrevLogTerm:  prevLogTerm,
				// Entries:      entries,
				Entries: node.log[node.nextIndex[addr]:],
				// Entries:      []LogEntry{},
				LeaderCommit: node.commitIndex,
			}
			node.mu.Unlock()

			wg.Add(1)
			go func(addr net.Addr, request *AppendEntriesRequest) {
				defer wg.Done()
				for {
					response := node.sendRequest("RaftNode.AppendEntries", addr, request)

					if response == nil {
						log.Println("Unreachable address when sending heartbeat... (", addr.String(), ")")
						if len(request.Entries) != 0 {
							node.nextIndex[addr]++
						}
						break
					}

					var result AppendEntriesResponse
					err := json.Unmarshal(response, &result)
					if err != nil {
						log.Printf("Error unmarshalling response from %s: %v\n", addr, err)
						return
					}

					if result.Success {
						// Break if empty heartbeat, otherwise increment nextIndex and matchIndex
						if len(request.Entries) != 0 {
							node.nextIndex[addr]++
							node.matchIndex[addr]++
							responses <- result
						}
						// print match index
						break
					} else {
						// time.Sleep(100 * time.Millisecond)
						request.PrevLogIndex--
						node.nextIndex[addr]--
						if request.PrevLogIndex < 0 {
							request.PrevLogTerm = 0
						} else {
							request.PrevLogTerm = node.log[request.PrevLogIndex].Term
						}
					}

				}
			}(addr, request)
		}

		// Wait for all goroutines to finish and then close the channel
		go func() {
			wg.Wait()
			close(responses)
		}()
	}
}

func (node *RaftNode) RequestVote(args *RaftVoteRequest, reply *[]byte) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	// Initialize response map
	responseMap := map[string]interface{}{
		"term":        node.currentTerm,
		"voteGranted": false,
	}

	// Return false if the candidate's term is less than the current term
	if args.Term < node.currentTerm {
		log.Printf("[%d, %s] Rejecting vote... (candidate term is less than current term) [%d vs %d]\n", node.nodeType, node.address.String(), args.Term, node.currentTerm)
		responseMap["term"] = node.currentTerm
		responseMap["voteGranted"] = false

		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Fatalf("Error marshalling response: %v", err)
		}
		*reply = responseBytes

		return nil
	}

	// Handle log term definition when log is empty
	var lastLogTerm int
	if len(node.log) == 0 {
		lastLogTerm = 0
	} else {
		lastLogTerm = node.log[len(node.log)-1].Term
	}

	// Grant vote if candidate's term is equal to current term and candidate's log is at least as up-to-date as receiver's log
	if (node.votedFor == nil || node.votedFor.String() == args.CandidateId.String()) && (args.LastLogIndex >= len(node.log)-1 && args.LastLogTerm >= lastLogTerm) {
		log.Printf("[%d, %s] Voting for %s...\n", node.nodeType, node.address.String(), args.CandidateId.String())
		node.votedFor = args.CandidateId
		responseMap["term"] = node.currentTerm
		responseMap["voteGranted"] = true

		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Fatalf("Error marshalling response: %v", err)
		}
		*reply = responseBytes

		return nil
	} else { // Reject vote if already voted or last log not matched
		log.Printf("[%d, %s] Rejecting vote... (Already voted or last log not matched)\n", node.nodeType, node.address.String())
		responseMap["term"] = node.currentTerm
		responseMap["voteGranted"] = false

		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Fatalf("Error marshalling response: %v", err)
		}
		*reply = responseBytes

		return nil
	}
}

func (node *RaftNode) AppendEntries(args *AppendEntriesRequest, reply *[]byte) error {
	node.mu.Lock()

	responseMap := map[string]interface{}{
		"term":    node.currentTerm,
		"success": true,
	}

	if args.Term < node.currentTerm { // Reject AppendEntries if term is less than current term
		log.Printf("[%d, %s] Rejecting AppendEntries from %s... (Term is less than current term)\n", node.nodeType, node.address.String(), args.LeaderId.String())
		responseMap["term"] = node.currentTerm
		responseMap["success"] = false

		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Fatalf("Error marshalling response: %v", err)
		}
		*reply = responseBytes
		node.mu.Unlock()

		return nil
	}

	// fmt.Println("Args prev log index:", args.PrevLogIndex)
	// fmt.Println("Args prev log term:", args.PrevLogTerm)

	if args.PrevLogIndex == -1 { // Check if leader log empty
		// Pass
	} else if len(node.log)-1 < args.PrevLogIndex { // Reject AppendEntries if log is shorter
		log.Printf("[%d, %s] Rejecting AppendEntries from %s... (Log is shorter)\n", node.nodeType, node.address.String(), args.LeaderId.String())
		responseMap["term"] = node.currentTerm
		responseMap["success"] = false

		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Printf("Error marshalling response: %v\n", err)
		}
		*reply = responseBytes
		node.mu.Unlock()

		return nil
	} else if node.log[args.PrevLogIndex].Term != args.PrevLogTerm { // Reject AppendEntries if term mismatch
		log.Printf("[%d, %s] Rejecting AppendEntries from %s... (Term mismatch)\n", node.nodeType, node.address.String(), args.LeaderId.String())
		responseMap["term"] = node.currentTerm
		responseMap["success"] = false

		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Fatalf("Error marshalling response: %v", err)
		}
		*reply = responseBytes
		node.mu.Unlock()

		return nil
	}

	// Below this line, AppendEntries will return success

	// Print log entries
	// fmt.Println("Log entries args:", args.Entries)

	if len(args.Entries) == 0 { // Empty AppendEntries (only heartbeat)
		log.Printf("[%d, %s] Received empty AppendEntries (heartbeat) from %s...\n", node.nodeType, node.address.String(), args.LeaderId.String())
	} else { // AppendEntries with entries
		node.log = node.log[:args.PrevLogIndex+1]
		node.log = append(node.log, args.Entries...)

		log.Printf("[%d, %s] Appending entries from %s successfully...\n", node.nodeType, node.address.String(), args.LeaderId.String())
	}

	// Update contact address
	tcpAddr, ok := args.LeaderId.(*net.TCPAddr)
	if !ok {
		log.Printf("Error converting address to TCP address of leader\n")
	} else {
		(*node.contactAddr).(*net.TCPAddr).IP = tcpAddr.IP
		(*node.contactAddr).(*net.TCPAddr).Port = tcpAddr.Port
	}

	if args.LeaderCommit > node.commitIndex {
		log.Printf("[%d, %s] Updating commit index from %d to %d...\n", node.nodeType, node.address.String(), node.commitIndex, min(args.LeaderCommit, len(node.log)-1))
		node.commitIndex = min(args.LeaderCommit, len(node.log)-1)
	}

	node.mu.Unlock()

	if node.commitIndex > node.lastApplied {
		res, ok := node.commit()
		if ok {
			log.Printf("[%d, %s] Applied command: %s\n", node.nodeType, node.address.String(), res)
		} else {
			log.Printf("[%d, %s] Failed to apply command\n", node.nodeType, node.address.String())
		}
	}

	node.mu.Lock()
	defer node.mu.Unlock()

	// Reset election timeout
	node.electionTimeout.Reset(time.Duration(ElectionTimeoutMin.Nanoseconds()+rand.Int63n(ElectionTimeoutMax.Nanoseconds()-ElectionTimeoutMin.Nanoseconds())) * time.Nanosecond)

	responseBytes, err := json.Marshal(responseMap)
	if err != nil {
		log.Fatalf("Error marshalling response: %v", err)
	}
	*reply = responseBytes

	return nil
}

func (node *RaftNode) startElection() {
	if node.nodeType == LEADER {
		return
	}
	node.mu.Lock()

	log.Println("Starting election...")
	node.nodeType = CANDIDATE
	node.electionTerm++
	node.currentTerm = node.electionTerm
	node.clusterLeader = nil

	node.mu.Unlock()

	voteCount := 1

	for _, addr := range node.clusterAddrList {
		if addr.String() == node.address.String() || addr.String() == (*node.contactAddr).String() {
			continue
		}
		go func(addr net.Addr) {
			log.Printf("[%s] Requesting vote\n", node.address)

			var lastLogTerm int
			if len(node.log) == 0 {
				lastLogTerm = 0
			} else {
				lastLogTerm = node.log[len(node.log)-1].Term
			}
			response := node.sendRequest("RaftNode.RequestVote", addr, RaftVoteRequest{
				Term:         node.electionTerm,
				CandidateId:  node.address,
				LastLogIndex: len(node.log) - 1,
				LastLogTerm:  lastLogTerm,
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
				tcpAddr, err := net.ResolveTCPAddr("tcp", node.clusterLeader.String())
				if err != nil {
					log.Fatal(err)
				}
				(*node.contactAddr).(*net.TCPAddr).IP = tcpAddr.IP
				(*node.contactAddr).(*net.TCPAddr).Port = tcpAddr.Port
				return
			}

			if result.VoteGranted {
				voteCount++
				if voteCount > len(node.clusterAddrList)/2 {
					log.Printf("[%s] Election won! %d votes received\n", node.address, voteCount)
					node.mu.Lock()
					defer node.mu.Unlock()
					node.nodeType = LEADER
					tcpAddr, ok := node.address.(*net.TCPAddr)
					if !ok {
						log.Printf("Error converting address to TCP address")
					}
					node.clusterLeader = tcpAddr
					node.contactAddr = nil

					for _, addr := range node.clusterAddrList {
						if addr.String() == node.address.String() {
							continue
						}
						node.nextIndex[addr] = len(node.log)
						node.mu.Unlock()
						responseMatchIndex := node.sendRequest("RaftNode.CalculateHighestMatchIndex", addr, &node.log)
						node.mu.Lock()
						var result map[string]interface{}
						err := json.Unmarshal(responseMatchIndex, &result)
						if err != nil {
							log.Printf("Error unmarshalling response: %v", err)
						}
						if result == nil {
							log.Printf("Unreachable address %s\n", addr.String())
							node.matchIndex[addr] = -1
							continue
						}
						node.matchIndex[addr] = int(result["highestMatchIndex"].(float64))

					}
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
			continue
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

	// append if not already in the list
	found := false
	for _, addr := range node.clusterAddrList {
		if addr.String() == args.String() {
			found = true
			break
		}
	}

	if !found {
		node.clusterAddrList = append(node.clusterAddrList, args)
	}

	for _, addr := range node.clusterAddrList {
		if addr.String() != node.address.String() && addr.String() != args.String() {
			go node.sendRequest("RaftNode.UpdateMembership", addr, args)
		}
	}

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
		return nil
	}
	*reply = responseBytes

	return nil
}

func (node *RaftNode) UpdateMembership(args *net.TCPAddr, reply *[]byte) error {
	node.mu.Lock()
	defer node.mu.Unlock()

	// append if not already in the list
	found := false
	for _, addr := range node.clusterAddrList {
		if addr.String() == args.String() {
			found = true
			break
		}
	}

	if !found {
		node.clusterAddrList = append(node.clusterAddrList, args)
	}

	return nil
}

func (node *RaftNode) CalculateHighestMatchIndex(args *[]LogEntry, reply *[]byte) error {
	node.mu.Lock()
	defer node.mu.Unlock()
	minLength := min(len(*args), len(node.log))

	highestMatchIndex := -1
	for i := 0; i < minLength; i++ {
		if (*args)[i].Term == node.log[i].Term && (*args)[i].Command == node.log[i].Command {
			highestMatchIndex++
		}
	}

	responseMap := map[string]interface{}{
		"highestMatchIndex": highestMatchIndex,
	}

	responseBytes, err := json.Marshal(responseMap)
	if err != nil {
		log.Fatalf("Error marshalling response: %v", err)
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

func (node *RaftNode) commit() (string, bool) {
	node.mu.Lock()
	defer node.mu.Unlock()

	var res string
	ok := false

	if node.commitIndex > node.lastApplied {
		node.lastApplied++
		entry := node.log[node.lastApplied]
		res, ok = node.apply(entry.Command)
	}

	return res, ok
}

func (node *RaftNode) apply(command string) (string, bool) {
	// parse command into each words (split by space)
	parts := strings.Fields(command)
	if len(parts) == 0 {
		log.Println("Empty command")
		return "", false
	}

	switch parts[0] {
	case "get":
		if len(parts) < 2 {
			log.Println("Not enough arguments for get")
			return "", false
		}
		return node.app.Get(parts[1]), true
	case "strlen":
		if len(parts) < 2 {
			log.Println("Not enough arguments for strlen")
			return "", false
		}
		return strconv.Itoa(node.app.Len(parts[1])), true
	case "del":
		if len(parts) < 2 {
			log.Println("Not enough arguments for del")
			return "", false
		}
		return node.app.Delete(parts[1]), true
	case "set":
		var value string
		if len(parts) < 2 {
			log.Println("Not enough arguments for set")
			return "", false
		}
		if len(parts) < 3 {
			value = ""
		} else {
			value = parts[2]
		}
		node.app.Set(parts[1], value)
		return "OK", true
	case "append":
		if len(parts) < 3 {
			log.Println("Not enough arguments for append")
			return "", false
		}
		node.app.Append(parts[1], parts[2])
		return "OK", true
	default:
		log.Printf("Unknown command: %s\n", parts[0])
		return "", false
	}
}

func (node *RaftNode) Execute(args string, reply *[]byte) error {
	if node.nodeType != LEADER { // only leader may accept client requests
		responseMap := map[string]interface{}{
			"error":      "Not leader",
			"leaderAddr": node.clusterLeader.String(),
		}
		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Fatalf("Error marshalling response: %v", err)
		}
		*reply = responseBytes
		return nil
	}

	// Append the command to the log
	node.mu.Lock()
	node.log = append(node.log, LogEntry{
		Term:    node.currentTerm,
		Command: args,
	})
	node.mu.Unlock()

	// Send AppendEntries to all nodes in the cluster
	responses := make(chan AppendEntriesResponse, len(node.clusterAddrList))
	var wg sync.WaitGroup

	for _, addr := range node.clusterAddrList {
		if addr.String() == node.address.String() {
			continue
		}

		var prevLogTerm int
		if len(node.log)-2 < 0 {
			prevLogTerm = 0 // 0 means empty log
		} else {
			prevLogTerm = node.log[len(node.log)-2].Term
		}

		request := &AppendEntriesRequest{
			Term:         node.currentTerm,
			LeaderId:     node.address,
			PrevLogIndex: len(node.log) - 2,
			PrevLogTerm:  prevLogTerm,
			Entries:      node.log[len(node.log)-1:],
			LeaderCommit: node.commitIndex,
		}

		wg.Add(1)
		go func(addr net.Addr, request *AppendEntriesRequest) {
			defer wg.Done()
			for {
				response := node.sendRequest("RaftNode.AppendEntries", addr, request)

				var result AppendEntriesResponse
				err := json.Unmarshal(response, &result)
				if err != nil {
					log.Printf("Error unmarshalling response from %s: %v\n", addr, err)
					return
				}
				node.mu.Lock()
				if result.Success {
					node.nextIndex[addr]++
					node.matchIndex[addr]++
					responses <- result
					node.mu.Unlock()
					break
				} else {
					// time.Sleep(100 * time.Millisecond)
					request.PrevLogIndex--
					node.nextIndex[addr]--
					if request.PrevLogIndex < 0 {
						request.PrevLogTerm = 0
					} else {
						request.PrevLogTerm = node.log[request.PrevLogIndex].Term
					}
				}
				node.mu.Unlock()

			}
		}(addr, request)
	}

	// Wait for all goroutines to finish and then close the channel
	go func() {
		wg.Wait()
		close(responses)
	}()

	// If majority ACK received, commit the log

	// If there is only one node in the cluster, immediately commit the log
	if len(node.clusterAddrList) == 1 {
		node.commitIndex = len(node.log) - 1
		res, ok := node.commit()
		responseMap := map[string]interface{}{
			"result": res,
			"ok":     ok,
		}
		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Printf("Error marshalling response: %v", err)
		}
		*reply = responseBytes
		return nil
	}

	successCount := 1
	for response := range responses {
		if response.Success {
			successCount++
			if successCount > len(node.clusterAddrList)/2 {
				// Check if there exists an N such that N > commitIndex, a majority of matchIndex[i] >= N, and log[N].term == currentTerm
				for N := node.commitIndex + 1; N < len(node.log); N++ {
					node.mu.Lock()
					matchCount := 1
					for _, matchIndex := range node.matchIndex {
						if matchIndex >= N {
							matchCount++
						}
					}
					if matchCount > len(node.clusterAddrList)/2 && node.log[N].Term == node.currentTerm {
						// Set commitIndex = N
						node.commitIndex = N
						break
					}
				}
				node.mu.Unlock()

				// Commit the log
				res, ok := node.commit()

				// Send response to client
				responseMap := map[string]interface{}{
					"result": res,
					"ok":     ok,
				}

				responseBytes, err := json.Marshal(responseMap)
				if err != nil {
					log.Fatalf("Error marshalling response: %v", err)
				}

				*reply = responseBytes

				erra := json.Unmarshal(responseBytes, &responseMap)
				if erra != nil {
					log.Fatalf("Error unmarshalling response: %v", erra)
				}
				break
			}
		}
	}

	return nil
}

func (node *RaftNode) RequestLog(args string, reply *[]byte) error {
	if node.nodeType != LEADER { // only leader may accept client requests
		responseMap := map[string]interface{}{
			"error":      "Not leader",
			"leaderAddr": node.clusterLeader.String(),
		}
		responseBytes, err := json.Marshal(responseMap)
		if err != nil {
			log.Fatalf("Error marshalling response: %v", err)
		}
		*reply = responseBytes
		return nil
	}

	log.Println("[Leader] Client requesting log...")

	// Send log to client
	responseMap := map[string]interface{}{
		"log": node.log,
	}

	responseBytes, err := json.Marshal(responseMap)
	if err != nil {
		log.Printf("Error marshalling response: %v", err)
		return nil
	}
	*reply = responseBytes

	return nil
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
