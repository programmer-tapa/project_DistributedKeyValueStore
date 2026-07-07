package core

// --- RSM (Replicated State Machine) Interfaces ---

// StateMachine defines the contract that application-level servers
// (KVServer, ShardGroup) must implement to be replicated by the RSM.
//
// The RSM calls DoOp for each committed log entry, and Snapshot/Restore
// for log compaction.
type StateMachine interface {
	// DoOp applies a committed operation and returns the result.
	// The op is the same value that was passed to RSM.Submit().
	DoOp(op interface{}) interface{}

	// Snapshot serializes the current application state for log compaction.
	Snapshot() []byte

	// Restore deserializes a snapshot to replace the current state.
	// Snapshots only advance state forward; they never regress.
	Restore(snapshot []byte)
}

// Op wraps a client command with a unique ID for RSM submission.
// The ID allows the RSM reader goroutine to match committed entries
// back to the waiting Submit() caller.
type Op struct {
	ID      int64       // Unique per Submit call (nonce)
	Payload interface{} // PutArgs, GetArgs, or shard migration commands
}
