package kvsrv

import (
	"bytes"
	"encoding/gob"
	"sync"

	"dkv/internal/core"
)

func init() {
	gob.Register(core.PutArgs{})
	gob.Register(core.GetArgs{})
	gob.Register(core.PutReply{})
	gob.Register(core.GetReply{})
	gob.Register(core.VersionedValue{})
}

// KVServer holds the in-memory key/value state.
// Implements core.StateMachine so it can be replicated by RSM.
type KVServer struct {
	mu    sync.Mutex
	store map[string]core.VersionedValue
}

// New creates a KVServer with an empty store.
func New() *KVServer {
	return &KVServer{
		store: make(map[string]core.VersionedValue),
	}
}

// Get handles a Get RPC — returns the value and version for a key.
func (kv *KVServer) Get(args *core.GetArgs, reply *core.GetReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	val, ok := kv.store[args.Key]
	if !ok {
		reply.Err = core.ErrNoKey
		return nil
	}
	reply.Value = val.Value
	reply.Version = val.Version
	reply.Err = core.OK
	return nil
}

// Put handles a Put RPC — installs value only if versions match.
func (kv *KVServer) Put(args *core.PutArgs, reply *core.PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	current, exists := kv.store[args.Key]
	if args.Version == 0 {
		if exists {
			reply.Err = core.ErrVersion
			return nil
		}
		kv.store[args.Key] = core.VersionedValue{
			Value:   args.Value,
			Version: 1,
		}
		reply.Err = core.OK
		return nil
	}

	if !exists {
		reply.Err = core.ErrNoKey
		return nil
	}

	if current.Version != args.Version {
		reply.Err = core.ErrVersion
		return nil
	}

	kv.store[args.Key] = core.VersionedValue{
		Value:   args.Value,
		Version: current.Version + 1,
	}
	reply.Err = core.OK
	return nil
}

// --- StateMachine interface (for RSM replication) ---

// DoOp applies a committed operation from the Raft log.
func (kv *KVServer) DoOp(op interface{}) interface{} {
	switch args := op.(type) {
	case core.GetArgs:
		var reply core.GetReply
		_ = kv.Get(&args, &reply)
		return reply
	case core.PutArgs:
		var reply core.PutReply
		_ = kv.Put(&args, &reply)
		return reply
	default:
		return nil
	}
}

// Snapshot serializes the KV state for log compaction.
func (kv *KVServer) Snapshot() []byte {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(kv.store); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// Restore replaces the KV state from a snapshot.
func (kv *KVServer) Restore(snapshot []byte) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if len(snapshot) == 0 {
		kv.store = make(map[string]core.VersionedValue)
		return
	}

	buf := bytes.NewBuffer(snapshot)
	dec := gob.NewDecoder(buf)
	var store map[string]core.VersionedValue
	if err := dec.Decode(&store); err != nil {
		panic(err)
	}
	kv.store = store
}
