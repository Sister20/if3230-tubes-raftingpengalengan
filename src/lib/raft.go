package lib

import "net"

type NodeType int

const (
	LEADER NodeType = iota + 1
	CANDIDATE
	FOLLOWER
)

type LogEntry struct {
	Term    int
	Command string
}

type Raft struct {
	NodeType NodeType
	Address  net.TCPAddr
	// persistent state
	Log         []LogEntry
	CurrentTerm int
	VotedFor    net.TCPAddr
	// volatile state
	CommitIndex int
	LastApplied int
	// volatile state for leaders
	NextIndex  map[string]int
	MatchIndex map[string]int
}
