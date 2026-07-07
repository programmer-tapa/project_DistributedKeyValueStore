package raft

import (
	"bytes"
	"encoding/gob"
	"log"

	"dkv/internal/core"
)

// serializeState serializes state for persistence (must be called with mu locked)
func (rf *Raft) serializeState() []byte {
	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)
	if err := e.Encode(rf.currentTerm); err != nil {
		log.Fatalf("encode currentTerm error: %v", err)
	}
	if err := e.Encode(rf.votedFor); err != nil {
		log.Fatalf("encode votedFor error: %v", err)
	}
	if err := e.Encode(rf.log); err != nil {
		log.Fatalf("encode log error: %v", err)
	}
	if err := e.Encode(rf.lastIncludedIndex); err != nil {
		log.Fatalf("encode lastIncludedIndex error: %v", err)
	}
	if err := e.Encode(rf.lastIncludedTerm); err != nil {
		log.Fatalf("encode lastIncludedTerm error: %v", err)
	}
	return w.Bytes()
}

// persist saves currentTerm, votedFor, and log to the Persister.
func (rf *Raft) persist() {
	rf.persister.Save(rf.serializeState(), rf.persister.ReadSnapshot())
}

// readPersist restores state from the Persister on startup.
func (rf *Raft) readPersist(data []byte) {
	if len(data) == 0 {
		return
	}
	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var logEntries []core.LogEntry
	var lastIncludedIndex int
	var lastIncludedTerm int
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&logEntries) != nil ||
		d.Decode(&lastIncludedIndex) != nil ||
		d.Decode(&lastIncludedTerm) != nil {
		log.Fatalf("readPersist decode error")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.log = logEntries
		rf.lastIncludedIndex = lastIncludedIndex
		rf.lastIncludedTerm = lastIncludedTerm
		rf.commitIndex = lastIncludedIndex
		rf.lastApplied = lastIncludedIndex
	}
}

// sendSnapshotToPeer sends a snapshot to a lagging follower.
func (rf *Raft) sendSnapshotToPeer(peerIdx int, peerAddr string, term int) {
	rf.mu.Lock()
	if rf.role != core.Leader || rf.currentTerm != term {
		rf.mu.Unlock()
		return
	}
	args := core.InstallSnapshotArgs{
		Term:              rf.currentTerm,
		LeaderID:          rf.me,
		LastIncludedIndex: rf.lastIncludedIndex,
		LastIncludedTerm:  rf.lastIncludedTerm,
		Data:              rf.persister.ReadSnapshot(),
	}
	rf.mu.Unlock()

	var reply core.InstallSnapshotReply
	ok := rf.network.Call(peerAddr, "Raft.InstallSnapshot", &args, &reply)
	if !ok {
		return
	}

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.role != core.Leader || rf.currentTerm != term {
		return
	}

	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		rf.role = core.Follower
		rf.votedFor = -1
		rf.persist()
		rf.resetElectionTimeout()
		return
	}

	if args.LastIncludedIndex > rf.matchIndex[peerIdx] {
		rf.matchIndex[peerIdx] = args.LastIncludedIndex
	}
	rf.nextIndex[peerIdx] = rf.matchIndex[peerIdx] + 1
}

// InstallSnapshot processes an incoming InstallSnapshot RPC.
func (rf *Raft) InstallSnapshot(args *core.InstallSnapshotArgs, reply *core.InstallSnapshotReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm

	if args.Term < rf.currentTerm {
		return nil
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = core.Follower
		rf.votedFor = -1
		rf.persist()
	} else if rf.role == core.Candidate {
		rf.role = core.Follower
	}

	rf.resetElectionTimeout()

	if args.LastIncludedIndex <= rf.lastIncludedIndex {
		return nil
	}

	lastLogIdx := rf.getLastLogIndex()
	if args.LastIncludedIndex <= lastLogIdx && rf.getLogTerm(args.LastIncludedIndex) == args.LastIncludedTerm {
		rf.log = rf.log[args.LastIncludedIndex-rf.lastIncludedIndex:]
		rf.log[0] = core.LogEntry{Term: args.LastIncludedTerm, Index: args.LastIncludedIndex}
	} else {
		rf.log = make([]core.LogEntry, 1)
		rf.log[0] = core.LogEntry{Term: args.LastIncludedTerm, Index: args.LastIncludedIndex}
	}

	rf.lastIncludedIndex = args.LastIncludedIndex
	rf.lastIncludedTerm = args.LastIncludedTerm

	if rf.commitIndex < args.LastIncludedIndex {
		rf.commitIndex = args.LastIncludedIndex
	}
	if rf.lastApplied < args.LastIncludedIndex {
		rf.lastApplied = args.LastIncludedIndex
	}

	rf.persister.Save(rf.serializeState(), args.Data)

	rf.pendingSnapshot = args.Data
	rf.pendingSnapshotIndex = args.LastIncludedIndex
	rf.pendingSnapshotTerm = args.LastIncludedTerm
	rf.pendingSnapshotValid = true

	rf.applyCond.Broadcast()
	return nil
}
