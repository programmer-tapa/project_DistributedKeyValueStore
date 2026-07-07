package raft

import (
	"time"

	"dkv/internal/core"
)

// replicationTicker periodically replicates log entries or sends snapshots to a follower.
func (rf *Raft) replicationTicker(peerIdx int, peerAddr string, term int) {
	for !rf.killed() {
		rf.mu.Lock()
		if rf.role != core.Leader || rf.currentTerm != term {
			rf.mu.Unlock()
			return
		}
		if rf.nextIndex[peerIdx] <= rf.lastIncludedIndex {
			go rf.sendSnapshotToPeer(peerIdx, peerAddr, term)
		} else {
			go rf.sendAppendEntriesToPeer(peerIdx, peerAddr, term)
		}
		rf.mu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}

// replicateToAll triggers replication to all followers immediately.
func (rf *Raft) replicateToAll() {
	rf.mu.Lock()
	if rf.role != core.Leader {
		rf.mu.Unlock()
		return
	}
	term := rf.currentTerm
	for peerIdx, peerAddr := range rf.peers {
		if peerIdx != rf.me {
			if rf.nextIndex[peerIdx] <= rf.lastIncludedIndex {
				go rf.sendSnapshotToPeer(peerIdx, peerAddr, term)
			} else {
				go rf.sendAppendEntriesToPeer(peerIdx, peerAddr, term)
			}
		}
	}
	rf.mu.Unlock()
}

// sendAppendEntriesToPeer constructs and sends AppendEntries RPC to a single follower.
func (rf *Raft) sendAppendEntriesToPeer(peerIdx int, peerAddr string, term int) {
	rf.mu.Lock()
	if rf.role != core.Leader || rf.currentTerm != term {
		rf.mu.Unlock()
		return
	}

	prevLogIndex := rf.nextIndex[peerIdx] - 1
	if prevLogIndex < rf.lastIncludedIndex {
		rf.mu.Unlock()
		return
	}
	prevLogTerm := rf.getLogTerm(prevLogIndex)
	entries := make([]core.LogEntry, rf.getLastLogIndex()-prevLogIndex)
	copy(entries, rf.log[prevLogIndex+1-rf.lastIncludedIndex:])

	args := core.AppendEntriesArgs{
		Term:         rf.currentTerm,
		LeaderID:     rf.me,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: rf.commitIndex,
	}
	rf.mu.Unlock()

	var reply core.AppendEntriesReply
	ok := rf.network.Call(peerAddr, "Raft.AppendEntries", &args, &reply)
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

	if reply.Success {
		match := args.PrevLogIndex + len(args.Entries)
		if match > rf.matchIndex[peerIdx] {
			rf.matchIndex[peerIdx] = match
		}
		rf.nextIndex[peerIdx] = rf.matchIndex[peerIdx] + 1

		for n := rf.getLastLogIndex(); n > rf.commitIndex; n-- {
			if rf.getLogTerm(n) == rf.currentTerm {
				count := 1
				for p := range rf.peers {
					if p != rf.me && rf.matchIndex[p] >= n {
						count++
					}
				}
				if count > len(rf.peers)/2 {
					rf.commitIndex = n
					rf.applyCond.Broadcast()
					break
				}
			}
		}
	} else {
		if reply.XTerm == -1 {
			rf.nextIndex[peerIdx] = reply.XLen
		} else {
			lastIdxWithTerm := -1
			lastLogIdx := rf.getLastLogIndex()
			for i := lastLogIdx; i >= rf.lastIncludedIndex; i-- {
				if rf.getLogTerm(i) == reply.XTerm {
					lastIdxWithTerm = i
					break
				}
			}
			if lastIdxWithTerm != -1 {
				rf.nextIndex[peerIdx] = lastIdxWithTerm + 1
			} else {
				rf.nextIndex[peerIdx] = reply.XIndex
			}
		}
		if rf.nextIndex[peerIdx] < 1 {
			rf.nextIndex[peerIdx] = 1
		}
	}
}

// AppendEntries processes an incoming AppendEntries RPC.
func (rf *Raft) AppendEntries(args *core.AppendEntriesArgs, reply *core.AppendEntriesReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.Term = rf.currentTerm
	reply.Success = false

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

	lastLogIdx := rf.getLastLogIndex()
	if args.PrevLogIndex > lastLogIdx {
		reply.XTerm = -1
		reply.XIndex = -1
		reply.XLen = lastLogIdx + 1
		return nil
	}

	if args.PrevLogIndex < rf.lastIncludedIndex {
		reply.XTerm = -1
		reply.XIndex = -1
		reply.XLen = rf.lastIncludedIndex + 1
		return nil
	}

	if rf.getLogTerm(args.PrevLogIndex) != args.PrevLogTerm {
		reply.XTerm = rf.getLogTerm(args.PrevLogIndex)
		firstIdx := args.PrevLogIndex
		for i := args.PrevLogIndex; i >= rf.lastIncludedIndex; i-- {
			if rf.getLogTerm(i) == reply.XTerm {
				firstIdx = i
			} else {
				break
			}
		}
		reply.XIndex = firstIdx
		reply.XLen = lastLogIdx + 1
		return nil
	}

	reply.Success = true

	for i, entry := range args.Entries {
		idx := args.PrevLogIndex + 1 + i
		if idx <= lastLogIdx {
			if rf.getLogTerm(idx) != entry.Term {
				rf.log = rf.log[:idx-rf.lastIncludedIndex]
				rf.log = append(rf.log, entry)
				rf.persist()
				lastLogIdx = rf.getLastLogIndex()
			}
		} else {
			rf.log = append(rf.log, entry)
			rf.persist()
			lastLogIdx = rf.getLastLogIndex()
		}
	}

	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = args.LeaderCommit
		if lastLogIdx < rf.commitIndex {
			rf.commitIndex = lastLogIdx
		}
		rf.applyCond.Broadcast()
	}
	return nil
}

// applier publishes committed entries and snapshots sequentially to the state machine.
func (rf *Raft) applier() {
	for !rf.killed() {
		rf.mu.Lock()
		for rf.commitIndex <= rf.lastApplied && !rf.pendingSnapshotValid && !rf.killed() {
			rf.applyCond.Wait()
		}

		if rf.killed() {
			rf.mu.Unlock()
			return
		}

		if rf.pendingSnapshotValid {
			msg := core.ApplyMsg{
				SnapshotValid: true,
				Snapshot:      rf.pendingSnapshot,
				SnapshotIndex: rf.pendingSnapshotIndex,
				SnapshotTerm:  rf.pendingSnapshotTerm,
			}
			rf.pendingSnapshotValid = false
			rf.pendingSnapshot = nil
			rf.mu.Unlock()
			rf.applyCh <- msg
			continue
		}

		msgs := make([]core.ApplyMsg, 0)
		for rf.lastApplied < rf.commitIndex {
			rf.lastApplied++
			idx := rf.lastApplied
			if idx <= rf.lastIncludedIndex {
				continue
			}
			entry := rf.getLogEntry(idx)
			msgs = append(msgs, core.ApplyMsg{
				CommandValid: true,
				Command:      entry.Command,
				CommandIndex: idx,
			})
		}
		rf.mu.Unlock()

		for _, msg := range msgs {
			rf.applyCh <- msg
		}
	}
}
