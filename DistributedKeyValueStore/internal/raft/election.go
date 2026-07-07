package raft

import (
	"time"

	"dkv/internal/core"
)

// ticker runs the election timeout / heartbeat loop.
func (rf *Raft) ticker() {
	for !rf.killed() {
		rf.mu.Lock()
		if rf.role != core.Leader && time.Since(rf.lastReset) > rf.electionTimeout {
			go rf.startElection()
		}
		rf.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
}

// startElection transitions to Candidate and sends RequestVote RPCs.
func (rf *Raft) startElection() {
	rf.mu.Lock()
	rf.currentTerm++
	rf.role = core.Candidate
	rf.votedFor = rf.me
	rf.persist()
	rf.resetElectionTimeout()

	term := rf.currentTerm
	votes := 1
	me := rf.me
	lastLogIndex := rf.getLastLogIndex()
	lastLogTerm := rf.getLastLogTerm()
	rf.mu.Unlock()

	for peerIdx, peerAddr := range rf.peers {
		if peerIdx == me {
			continue
		}
		go func(pIdx int, pAddr string) {
			args := core.RequestVoteArgs{
				Term:         term,
				CandidateID:  me,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}
			var reply core.RequestVoteReply
			ok := rf.network.Call(pAddr, "Raft.RequestVote", &args, &reply)
			if !ok {
				return
			}

			rf.mu.Lock()
			defer rf.mu.Unlock()

			if rf.role != core.Candidate || rf.currentTerm != term {
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

			if reply.VoteGranted {
				votes++
				if votes > len(rf.peers)/2 {
					rf.transitionToLeader()
				}
			}
		}(peerIdx, peerAddr)
	}
}

// transitionToLeader initializes leader volatile state.
func (rf *Raft) transitionToLeader() {
	rf.role = core.Leader
	lastIndex := rf.getLastLogIndex()
	for i := range rf.peers {
		rf.nextIndex[i] = lastIndex + 1
		rf.matchIndex[i] = 0
	}
	for peerIdx, peerAddr := range rf.peers {
		if peerIdx != rf.me {
			go rf.replicationTicker(peerIdx, peerAddr, rf.currentTerm)
		}
	}
}

// RequestVote processes an incoming RequestVote RPC.
func (rf *Raft) RequestVote(args *core.RequestVoteArgs, reply *core.RequestVoteReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	if args.Term < rf.currentTerm {
		return nil
	}

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.role = core.Follower
		rf.votedFor = -1
		rf.persist()
	}

	lastLogTerm := rf.getLastLogTerm()
	lastLogIndex := rf.getLastLogIndex()
	logUpToDate := false
	if args.LastLogTerm > lastLogTerm {
		logUpToDate = true
	} else if args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex {
		logUpToDate = true
	}

	if (rf.votedFor == -1 || rf.votedFor == args.CandidateID) && logUpToDate {
		reply.VoteGranted = true
		rf.votedFor = args.CandidateID
		rf.persist()
		rf.resetElectionTimeout()
	}
	return nil
}
