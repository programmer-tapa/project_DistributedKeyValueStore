// Package raft implements the Raft consensus protocol.
//
// This is the heart of the system — a replicated log that guarantees:
//   - Safety: At most one leader per term; committed entries are never lost
//   - Liveness: A leader is elected within 5s if a majority is reachable
//
// The implementation follows the extended Raft paper (Ongaro & Ousterhout)
// and is structured in four progressive parts:
//   - 3A: Leader election (RequestVote, heartbeats, randomized timeouts)
//   - 3B: Log replication (AppendEntries, commitment, applyCh delivery)
//   - 3C: Persistence (currentTerm, votedFor, log encoded via labgob)
//   - 3D: Log compaction (Snapshot, InstallSnapshot, trimmed log)
//
// Thread safety: All state is guarded by a single sync.Mutex.
// Communication: Raft peers communicate via the core.Network interface,
// allowing the same code to run over in-memory channels (tests) or gRPC (production).
package raft
