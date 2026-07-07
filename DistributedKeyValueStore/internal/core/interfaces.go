package core

// Persister defines the contract for durable state storage.
// Raft persists currentTerm, votedFor, log, and snapshots through this interface.
//
// Implementations:
//   - internal/persist/memory.go  → in-memory (for unit tests, mirrors lab's Persister)
//   - internal/persist/disk.go    → file-based (for production Docker deployments)
type Persister interface {
	// Save atomically persists Raft state and an optional snapshot.
	// raftstate is a GOB-encoded RaftPersistState.
	// snapshot may be nil if no snapshot exists.
	Save(raftstate []byte, snapshot []byte)

	// ReadRaftState returns the most recently persisted Raft state.
	ReadRaftState() []byte

	// ReadSnapshot returns the most recently persisted snapshot.
	ReadSnapshot() []byte

	// PersistBytes returns the size of the persisted Raft state in bytes.
	// Used by RSM to decide when to trigger log compaction.
	PersistBytes() int
}

// Network defines the transport abstraction for inter-node communication.
// This is the dependency inversion boundary that allows swapping between
// the in-memory test network (labrpc-style) and real TCP/gRPC.
//
// Implementations:
//   - internal/transport/local.go → in-memory channels (for unit tests)
//   - internal/transport/grpc.go  → gRPC over TCP (for production)
type Network interface {
	// Call sends an RPC to the specified peer and waits for a reply.
	// Returns false if the RPC was not delivered (timeout, partition, etc.).
	Call(peerAddr string, method string, args interface{}, reply interface{}) bool
}
