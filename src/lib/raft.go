package lib

type NodeType int

const (
	LEADER NodeType = iota
	CANDIDATE
	FOLLOWER
)
