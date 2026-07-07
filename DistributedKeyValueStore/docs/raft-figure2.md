# Raft Figure 2 Reference & Implementation Notes

This document acts as a checklist for the Raft state machine parameters and RPCs based on Figure 2 of the Raft paper.

## State

### Persistent State on All Servers
- `currentTerm`
- `votedFor`
- `log[]`

### Volatile State on All Servers
- `commitIndex`
- `lastApplied`

### Volatile State on Leaders
- `nextIndex[]`
- `matchIndex[]`

## AppendEntries RPC

- `term`
- `leaderId`
- `prevLogIndex`
- `prevLogTerm`
- `entries[]`
- `leaderCommit`

## RequestVote RPC

- `term`
- `candidateId`
- `lastLogIndex`
- `lastLogTerm`
