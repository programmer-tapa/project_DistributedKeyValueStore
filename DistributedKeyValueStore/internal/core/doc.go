// Package core defines the foundational domain types and interfaces for the
// distributed key/value store. This is the innermost architectural layer —
// it has ZERO external dependencies and ZERO knowledge of transport,
// persistence, or infrastructure.
//
// All other packages depend on core; core depends on nothing.
//
// Subsystems defined here:
//   - KV types: KeyValue, versioned Put/Get semantics, error codes
//   - Raft types: LogEntry, ApplyMsg, RaftState, peer state machine
//   - RSM types: StateMachine interface, Op wrapper
//   - Shard types: ShardConfig, shard assignment, migration protocol
//   - Transport: Network interface for dependency inversion
//   - Persistence: Persister interface for dependency inversion
package core
