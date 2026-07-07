package shardgrp

import (
	"dkv/internal/core"
)

// Clerk communicates with a specific ShardGroup replica group.
type Clerk struct {
	servers []string
	network core.Network
}

// NewClerk creates a new ShardGroup Clerk.
func NewClerk(servers []string, network core.Network) *Clerk {
	return &Clerk{
		servers: servers,
		network: network,
	}
}

// Get performs a read from the ShardGroup.
func (ck *Clerk) Get(args *core.GetArgs) core.GetReply {
	for {
		for _, srv := range ck.servers {
			var reply core.GetReply
			ok := ck.network.Call(srv, "ShardGroup.Get", args, &reply)
			if ok && reply.Err != core.ErrWrongLeader {
				return reply
			}
		}
	}
}

// Put performs a versioned write/update on the ShardGroup.
func (ck *Clerk) Put(args *core.PutArgs) core.PutReply {
	for {
		for _, srv := range ck.servers {
			var reply core.PutReply
			ok := ck.network.Call(srv, "ShardGroup.Put", args, &reply)
			if ok && reply.Err != core.ErrWrongLeader {
				return reply
			}
		}
	}
}

// FreezeShard sends the FreezeShard RPC to the group.
func (ck *Clerk) FreezeShard(args *core.FreezeShardArgs) core.FreezeShardReply {
	for {
		for _, srv := range ck.servers {
			var reply core.FreezeShardReply
			ok := ck.network.Call(srv, "ShardGroup.FreezeShard", args, &reply)
			if ok && reply.Err != core.ErrWrongLeader {
				return reply
			}
		}
	}
}

// InstallShard sends the InstallShard RPC to the group.
func (ck *Clerk) InstallShard(args *core.InstallShardArgs) core.InstallShardReply {
	for {
		for _, srv := range ck.servers {
			var reply core.InstallShardReply
			ok := ck.network.Call(srv, "ShardGroup.InstallShard", args, &reply)
			if ok && reply.Err != core.ErrWrongLeader {
				return reply
			}
		}
	}
}

// DeleteShard sends the DeleteShard RPC to the group.
func (ck *Clerk) DeleteShard(args *core.DeleteShardArgs) core.DeleteShardReply {
	for {
		for _, srv := range ck.servers {
			var reply core.DeleteShardReply
			ok := ck.network.Call(srv, "ShardGroup.DeleteShard", args, &reply)
			if ok && reply.Err != core.ErrWrongLeader {
				return reply
			}
		}
	}
}
