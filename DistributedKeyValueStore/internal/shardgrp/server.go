// Package shardgrp implements a shard-aware replica group.
package shardgrp

import (
	"bytes"
	"encoding/gob"
	"sync"

	"dkv/internal/core"
	"dkv/internal/rsm"
)

func init() {
	gob.Register(core.GetArgs{})
	gob.Register(core.PutArgs{})
	gob.Register(core.FreezeShardArgs{})
	gob.Register(core.InstallShardArgs{})
	gob.Register(core.DeleteShardArgs{})
	gob.Register(core.VersionedValue{})
	gob.Register(core.GetReply{})
	gob.Register(core.PutReply{})
	gob.Register(core.FreezeShardReply{})
	gob.Register(core.InstallShardReply{})
	gob.Register(core.DeleteShardReply{})
	gob.Register(ShardGroupSnapshot{})
}

type ShardGroupSnapshot struct {
	Shards     [core.NShards]map[string]core.VersionedValue
	Owned      [core.NShards]bool
	Frozen     [core.NShards]bool
	ConfigNums [core.NShards]int
}

// ShardGroup extends KVServer with shard-awareness.
// Implements core.StateMachine so it can be replicated by RSM.
type ShardGroup struct {
	mu   sync.Mutex
	gid  int
	rsm  *rsm.RSM

	shards     [core.NShards]map[string]core.VersionedValue
	owned      [core.NShards]bool
	frozen     [core.NShards]bool
	configNums [core.NShards]int
}

// New creates a ShardGroup with the given Group ID.
func New(gid int) *ShardGroup {
	sg := &ShardGroup{gid: gid}
	for i := 0; i < core.NShards; i++ {
		sg.shards[i] = make(map[string]core.VersionedValue)
	}
	return sg
}

func (sg *ShardGroup) SetRSM(r *rsm.RSM) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.rsm = r
}

// --- Client-facing KV operations ---

// Get serves a Get request, rejecting if the shard is not owned or frozen.
func (sg *ShardGroup) Get(args *core.GetArgs, reply *core.GetReply) error {
	sg.mu.Lock()
	r := sg.rsm
	sg.mu.Unlock()

	if r == nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}

	rep, err := r.Submit(*args)
	if err != nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}
	*reply = rep.(core.GetReply)
	return nil
}

// Put serves a Put request with shard ownership checks.
func (sg *ShardGroup) Put(args *core.PutArgs, reply *core.PutReply) error {
	sg.mu.Lock()
	r := sg.rsm
	sg.mu.Unlock()

	if r == nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}

	rep, err := r.Submit(*args)
	if err != nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}
	*reply = rep.(core.PutReply)
	return nil
}

// --- Shard Migration Handlers ---

// FreezeShard stops serving a shard and returns its data for migration.
func (sg *ShardGroup) FreezeShard(args *core.FreezeShardArgs, reply *core.FreezeShardReply) error {
	sg.mu.Lock()
	r := sg.rsm
	sg.mu.Unlock()

	if r == nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}

	rep, err := r.Submit(*args)
	if err != nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}
	*reply = rep.(core.FreezeShardReply)
	return nil
}

// InstallShard receives shard data from a migration and starts serving it.
func (sg *ShardGroup) InstallShard(args *core.InstallShardArgs, reply *core.InstallShardReply) error {
	sg.mu.Lock()
	r := sg.rsm
	sg.mu.Unlock()

	if r == nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}

	rep, err := r.Submit(*args)
	if err != nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}
	*reply = rep.(core.InstallShardReply)
	return nil
}

// DeleteShard discards a shard that has been successfully migrated away.
func (sg *ShardGroup) DeleteShard(args *core.DeleteShardArgs, reply *core.DeleteShardReply) error {
	sg.mu.Lock()
	r := sg.rsm
	sg.mu.Unlock()

	if r == nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}

	rep, err := r.Submit(*args)
	if err != nil {
		reply.Err = core.ErrWrongLeader
		return nil
	}
	*reply = rep.(core.DeleteShardReply)
	return nil
}

// --- StateMachine interface ---

func (sg *ShardGroup) DoOp(op interface{}) interface{} {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	switch args := op.(type) {
	case core.GetArgs:
		shard := core.Key2Shard(args.Key)
		var reply core.GetReply
		if !sg.owned[shard] {
			reply.Err = core.ErrWrongGroup
			return reply
		}
		if sg.frozen[shard] {
			reply.Err = core.ErrFrozen
			return reply
		}
		if sg.shards[shard] == nil {
			sg.shards[shard] = make(map[string]core.VersionedValue)
		}
		val, ok := sg.shards[shard][args.Key]
		if !ok {
			reply.Err = core.ErrNoKey
			return reply
		}
		reply.Value = val.Value
		reply.Version = val.Version
		reply.Err = core.OK
		return reply

	case core.PutArgs:
		shard := core.Key2Shard(args.Key)
		var reply core.PutReply
		if !sg.owned[shard] {
			reply.Err = core.ErrWrongGroup
			return reply
		}
		if sg.frozen[shard] {
			reply.Err = core.ErrFrozen
			return reply
		}
		if sg.shards[shard] == nil {
			sg.shards[shard] = make(map[string]core.VersionedValue)
		}

		current, exists := sg.shards[shard][args.Key]
		if args.Version == 0 {
			if exists {
				reply.Err = core.ErrVersion
				return reply
			}
			sg.shards[shard][args.Key] = core.VersionedValue{
				Value:   args.Value,
				Version: 1,
			}
			reply.Err = core.OK
			return reply
		}

		if !exists {
			reply.Err = core.ErrNoKey
			return reply
		}

		if current.Version != args.Version {
			reply.Err = core.ErrVersion
			return reply
		}

		sg.shards[shard][args.Key] = core.VersionedValue{
			Value:   args.Value,
			Version: current.Version + 1,
		}
		reply.Err = core.OK
		return reply

	case core.FreezeShardArgs:
		var reply core.FreezeShardReply
		shard := args.Shard
		if args.ConfigNum < sg.configNums[shard] {
			reply.Err = core.ErrVersion
			return reply
		}
		sg.configNums[shard] = args.ConfigNum
		sg.owned[shard] = false
		sg.frozen[shard] = true

		reply.Data = make(map[string]core.VersionedValue)
		if sg.shards[shard] != nil {
			for k, v := range sg.shards[shard] {
				reply.Data[k] = v
			}
		}
		reply.Err = core.OK
		return reply

	case core.InstallShardArgs:
		var reply core.InstallShardReply
		shard := args.Shard
		if args.ConfigNum < sg.configNums[shard] {
			reply.Err = core.ErrVersion
			return reply
		}
		sg.configNums[shard] = args.ConfigNum
		sg.owned[shard] = true
		sg.frozen[shard] = false

		sg.shards[shard] = make(map[string]core.VersionedValue)
		for k, v := range args.Data {
			sg.shards[shard][k] = v
		}
		reply.Err = core.OK
		return reply

	case core.DeleteShardArgs:
		var reply core.DeleteShardReply
		shard := args.Shard
		if args.ConfigNum < sg.configNums[shard] {
			reply.Err = core.ErrVersion
			return reply
		}
		sg.configNums[shard] = args.ConfigNum
		sg.owned[shard] = false
		sg.frozen[shard] = false
		sg.shards[shard] = make(map[string]core.VersionedValue)
		reply.Err = core.OK
		return reply
	}
	return nil
}

func (sg *ShardGroup) Snapshot() []byte {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	snap := ShardGroupSnapshot{
		Shards:     sg.shards,
		Owned:      sg.owned,
		Frozen:     sg.frozen,
		ConfigNums: sg.configNums,
	}
	if err := enc.Encode(snap); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func (sg *ShardGroup) Restore(snapshot []byte) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if len(snapshot) == 0 {
		for i := 0; i < core.NShards; i++ {
			sg.shards[i] = make(map[string]core.VersionedValue)
			sg.owned[i] = false
			sg.frozen[i] = false
			sg.configNums[i] = 0
		}
		return
	}

	buf := bytes.NewBuffer(snapshot)
	dec := gob.NewDecoder(buf)
	var snap ShardGroupSnapshot
	if err := dec.Decode(&snap); err != nil {
		panic(err)
	}
	sg.shards = snap.Shards
	sg.owned = snap.Owned
	sg.frozen = snap.Frozen
	sg.configNums = snap.ConfigNums
}
