package core

// --- KV Error Codes ---

// Err represents the result of a KV operation.
type Err string

const (
	OK         Err = "OK"
	ErrNoKey   Err = "ErrNoKey"
	ErrVersion Err = "ErrVersion"
	ErrMaybe   Err = "ErrMaybe" // ambiguous: Put may or may not have executed

	ErrWrongLeader Err = "ErrWrongLeader" // client should retry on another server
	ErrWrongGroup  Err = "ErrWrongGroup"  // shard not owned by this group
	ErrFrozen      Err = "ErrFrozen"      // shard is frozen during migration
)

// --- KV RPC Types ---

// PutArgs represents a versioned Put request.
type PutArgs struct {
	Key     string
	Value   string
	Version uint64 // Must match server's version; 0 = create new key
}

// PutReply is the server's response to a Put.
type PutReply struct {
	Err Err
}

// GetArgs represents a Get request.
type GetArgs struct {
	Key string
}

// GetReply is the server's response to a Get.
type GetReply struct {
	Err     Err
	Value   string
	Version uint64
}
