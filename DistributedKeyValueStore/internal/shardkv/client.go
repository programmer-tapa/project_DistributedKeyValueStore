// Package shardkv provides the top-level client that routes requests
// across shard groups using the shard controller.
package shardkv

import (
	"time"

	"dkv/internal/core"
	"dkv/internal/shardctrler"
	"dkv/internal/shardgrp"
)

// Clerk routes client requests to the correct shard group.
type Clerk struct {
	sc      *shardctrler.ShardController
	network core.Network
	config  core.ShardConfig
	clerks  map[int]*shardgrp.Clerk
}

// NewClerk creates a ShardKV Clerk connected to the shard controller.
func NewClerk(ctrlerAddr string, network core.Network) *Clerk {
	ck := &Clerk{
		sc:      shardctrler.New(ctrlerAddr, nil, network),
		network: network,
		clerks:  make(map[int]*shardgrp.Clerk),
	}
	if cfg, err := ck.sc.Query(); err == nil {
		ck.config = cfg
	}
	return ck
}

func (ck *Clerk) getGroupClerk(gid int) *shardgrp.Clerk {
	if gk, ok := ck.clerks[gid]; ok {
		return gk
	}
	servers := ck.config.Groups[gid]
	gk := shardgrp.NewClerk(servers, ck.network)
	ck.clerks[gid] = gk
	return gk
}

// Get fetches a key from the correct shard group.
func (ck *Clerk) Get(key string) (string, uint64, core.Err) {
	shard := core.Key2Shard(key)
	for {
		gid := ck.config.Shards[shard]
		if gid != 0 {
			gk := ck.getGroupClerk(gid)
			reply := gk.Get(&core.GetArgs{Key: key})
			if reply.Err == core.OK || reply.Err == core.ErrNoKey {
				return reply.Value, reply.Version, reply.Err
			}
		}
		if cfg, err := ck.sc.Query(); err == nil {
			ck.config = cfg
			ck.clerks = make(map[int]*shardgrp.Clerk)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Put writes a key to the correct shard group.
func (ck *Clerk) Put(key, value string, version uint64) core.Err {
	shard := core.Key2Shard(key)
	for {
		gid := ck.config.Shards[shard]
		if gid != 0 {
			gk := ck.getGroupClerk(gid)
			reply := gk.Put(&core.PutArgs{Key: key, Value: value, Version: version})
			if reply.Err == core.OK || reply.Err == core.ErrVersion || reply.Err == core.ErrNoKey {
				return reply.Err
			}
		}
		if cfg, err := ck.sc.Query(); err == nil {
			ck.config = cfg
			ck.clerks = make(map[int]*shardgrp.Clerk)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
