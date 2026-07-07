package kvsrv

import (
	"time"

	"dkv/internal/core"
)

// Clerk provides a client-side API for interacting with a KVServer.
// It handles RPC sending, retries on dropped messages, and ErrMaybe logic.
type Clerk struct {
	server  string
	network core.Network
}

// NewClerk creates a Clerk connected to the KVServer at the given address.
func NewClerk(serverAddr string, network core.Network) *Clerk {
	return &Clerk{
		server:  serverAddr,
		network: network,
	}
}

// Get fetches the current value and version for a key.
func (ck *Clerk) Get(key string) (string, uint64, core.Err) {
	args := core.GetArgs{Key: key}
	for {
		var reply core.GetReply
		ok := ck.network.Call(ck.server, "KVServer.Get", &args, &reply)
		if ok {
			return reply.Value, reply.Version, reply.Err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Put installs or replaces a value if version matches.
// Returns ErrMaybe if a retransmitted Put got ErrVersion (ambiguous outcome).
func (ck *Clerk) Put(key, value string, version uint64) core.Err {
	args := core.PutArgs{
		Key:     key,
		Value:   value,
		Version: version,
	}
	retried := false
	for {
		var reply core.PutReply
		ok := ck.network.Call(ck.server, "KVServer.Put", &args, &reply)
		if ok {
			if reply.Err == core.ErrVersion && retried {
				return core.ErrMaybe
			}
			return reply.Err
		}
		retried = true
		time.Sleep(100 * time.Millisecond)
	}
}
