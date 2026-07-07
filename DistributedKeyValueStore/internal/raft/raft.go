package raft

import (
	"encoding/gob"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"dkv/internal/core"
)

func init() {
	gob.Register(core.RequestVoteArgs{})
	gob.Register(core.RequestVoteReply{})
	gob.Register(core.AppendEntriesArgs{})
	gob.Register(core.AppendEntriesReply{})
	gob.Register(core.InstallSnapshotArgs{})
	gob.Register(core.InstallSnapshotReply{})
}

// Raft implements a single Raft consensus peer.
type Raft struct {
	mu        sync.Mutex
	peers     []string // peer addresses
	network   core.Network
	persister core.Persister
	me        int
	dead      int32

	// Persistent state
	currentTerm       int
	votedFor          int // -1 if none
	log               []core.LogEntry
	lastIncludedIndex int
	lastIncludedTerm  int

	// Volatile state
	commitIndex int
	lastApplied int
	role        core.RaftRole

	// Leader volatile state
	nextIndex  []int
	matchIndex []int

	// Election/Heartbeat timers and application channel
	lastReset       time.Time
	electionTimeout time.Duration
	applyCh         chan core.ApplyMsg
	applyCond       *sync.Cond

	// Snapshot buffering
	pendingSnapshot      []byte
	pendingSnapshotIndex int
	pendingSnapshotTerm  int
	pendingSnapshotValid bool
}

// Make creates and initializes a new Raft peer.
func Make(peers []string, me int, persister core.Persister, network core.Network, applyCh chan core.ApplyMsg) *Raft {
	rf := &Raft{
		peers:             peers,
		me:                me,
		persister:         persister,
		network:           network,
		applyCh:           applyCh,
		role:              core.Follower,
		votedFor:          -1,
		currentTerm:       0,
		lastIncludedIndex: 0,
		lastIncludedTerm:  0,
	}
	rf.applyCond = sync.NewCond(&rf.mu)
	rf.log = make([]core.LogEntry, 1)
	rf.log[0] = core.LogEntry{Term: 0, Index: 0}

	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	rf.resetElectionTimeout()

	// restore state
	rf.readPersist(persister.ReadRaftState())

	snapshot := persister.ReadSnapshot()
	if len(snapshot) > 0 {
		rf.commitIndex = rf.lastIncludedIndex
		rf.lastApplied = rf.lastIncludedIndex
	}

	go rf.applier()
	go rf.ticker()

	return rf
}

// Start proposes a new command to the Raft log.
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	index := -1
	term := rf.currentTerm
	isLeader := (rf.role == core.Leader)

	if isLeader {
		idx := rf.getLastLogIndex() + 1
		rf.log = append(rf.log, core.LogEntry{Command: command, Term: term, Index: idx})
		index = idx
		rf.persist()
		go rf.replicateToAll()
	}

	return index, term, isLeader
}

// GetState returns the current term and whether this peer is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.role == core.Leader
}

// Snapshot tells Raft to compact its log up to the given index.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if index <= rf.lastIncludedIndex {
		return
	}

	lastIncludedTerm := rf.getLogTerm(index)
	rf.log = rf.log[index-rf.lastIncludedIndex:]
	rf.log[0] = core.LogEntry{Term: lastIncludedTerm, Index: index}
	rf.lastIncludedIndex = index
	rf.lastIncludedTerm = lastIncludedTerm

	rf.persister.Save(rf.serializeState(), snapshot)
}

// PersistBytes returns the size of the persisted Raft state.
func (rf *Raft) PersistBytes() int {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.persister.PersistBytes()
}

// Kill terminates this Raft peer.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	rf.mu.Lock()
	rf.applyCond.Broadcast()
	rf.mu.Unlock()
}

func (rf *Raft) killed() bool {
	return atomic.LoadInt32(&rf.dead) == 1
}

// Log indexing helpers (must be called with mu locked)
func (rf *Raft) getLastLogIndex() int {
	return rf.lastIncludedIndex + len(rf.log) - 1
}

func (rf *Raft) getLastLogTerm() int {
	if len(rf.log) == 1 {
		return rf.lastIncludedTerm
	}
	return rf.log[len(rf.log)-1].Term
}

func (rf *Raft) getLogEntry(idx int) core.LogEntry {
	return rf.log[idx-rf.lastIncludedIndex]
}

func (rf *Raft) getLogTerm(idx int) int {
	if idx == rf.lastIncludedIndex {
		return rf.lastIncludedTerm
	}
	return rf.log[idx-rf.lastIncludedIndex].Term
}

func (rf *Raft) resetElectionTimeout() {
	rf.lastReset = time.Now()
	rf.electionTimeout = time.Duration(300+time.Duration(rand.Int63n(200))) * time.Millisecond
}
