package core

// --- Raft State Machine ---

// RaftRole represents the current role of a Raft peer.
type RaftRole int

const (
	Follower  RaftRole = iota
	Candidate
	Leader
)

// LogEntry is a single entry in the Raft replicated log.
type LogEntry struct {
	Term    int         // Term when entry was received by leader
	Index   int         // Position in the log (0-indexed with dummy entry at 0)
	Command interface{} // The client command (opaque to Raft)
}

// ApplyMsg is sent on the applyCh to deliver committed entries and snapshots
// to the service layer (RSM).
type ApplyMsg struct {
	// For normal committed commands:
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For snapshot delivery:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// --- Raft Persistent State ---
// These fields MUST survive crashes (Figure 2 of Raft paper):

// RaftPersistState holds the state that must be persisted to stable storage
// before responding to RPCs.
type RaftPersistState struct {
	CurrentTerm int
	VotedFor    int // -1 if none
	Log         []LogEntry
}

// --- Raft RPC Types ---

// RequestVoteArgs is sent by candidates to gather votes.
type RequestVoteArgs struct {
	Term         int // Candidate's term
	CandidateID  int // Candidate requesting vote
	LastLogIndex int // Index of candidate's last log entry
	LastLogTerm  int // Term of candidate's last log entry
}

// RequestVoteReply is the response to a RequestVote RPC.
type RequestVoteReply struct {
	Term        int  // CurrentTerm, for candidate to update itself
	VoteGranted bool // True means candidate received vote
}

// AppendEntriesArgs is sent by leaders to replicate log entries and heartbeats.
type AppendEntriesArgs struct {
	Term         int        // Leader's term
	LeaderID     int        // So followers can redirect clients
	PrevLogIndex int        // Index of log entry immediately preceding new ones
	PrevLogTerm  int        // Term of PrevLogIndex entry
	Entries      []LogEntry // Log entries to store (empty for heartbeat)
	LeaderCommit int        // Leader's commitIndex
}

// AppendEntriesReply is the response to an AppendEntries RPC.
type AppendEntriesReply struct {
	Term    int  // CurrentTerm, for leader to update itself
	Success bool // True if follower contained matching entry at PrevLogIndex

	// Optimization fields for fast log backtracking:
	XTerm  int // Term in the conflicting entry (if any)
	XIndex int // Index of first entry with XTerm (if any)
	XLen   int // Log length (for "log too short" case)
}

// InstallSnapshotArgs is sent by leaders to lagging followers.
type InstallSnapshotArgs struct {
	Term              int    // Leader's term
	LeaderID          int
	LastIncludedIndex int    // Index of last entry in the snapshot
	LastIncludedTerm  int    // Term of LastIncludedIndex
	Data              []byte // Raw snapshot bytes
}

// InstallSnapshotReply is the response to an InstallSnapshot RPC.
type InstallSnapshotReply struct {
	Term int // CurrentTerm, for leader to update itself
}
