package shardkv

import (
	"testing"
	"time"

	"dkv/internal/core"
	"dkv/internal/kvsrv"
	"dkv/internal/persist"
	"dkv/internal/raft"
	"dkv/internal/rsm"
	"dkv/internal/shardctrler"
	"dkv/internal/shardgrp"
	"dkv/internal/transport"
)

type testCluster struct {
	t            *testing.T
	net          *transport.LocalNetwork
	kvsrvServer  *kvsrv.KVServer
	kvsrvAddr    string
	controller   *shardctrler.ShardController
	groupServers map[int][]*shardgrp.ShardGroup
	groupRafts   map[int][]*raft.Raft
}

func makeTestCluster(t *testing.T) *testCluster {
	net := transport.NewLocalNetwork()
	tc := &testCluster{
		t:            t,
		net:          net,
		kvsrvAddr:    "config-store",
		groupServers: make(map[int][]*shardgrp.ShardGroup),
		groupRafts:   make(map[int][]*raft.Raft),
	}

	// 1. Start config storage (kvsrv)
	tc.kvsrvServer = kvsrv.New()
	net.Register(tc.kvsrvAddr, "KVServer", tc.kvsrvServer)

	// 2. Setup GIDs and groups
	// Group 1: servers g1-0, g1-1, g1-2
	// Group 2: servers g2-0, g2-1, g2-2
	gids := []int{101, 102}
	groups := map[int][]string{
		101: {"g1-0", "g1-1", "g1-2"},
		102: {"g2-0", "g2-1", "g2-2"},
	}

	// 3. Initialize Controller
	tc.controller = shardctrler.New(tc.kvsrvAddr, groups, net)

	// 4. Start ShardGroups with Raft/RSM
	for _, gid := range gids {
		servers := groups[gid]
		tc.groupServers[gid] = make([]*shardgrp.ShardGroup, len(servers))
		tc.groupRafts[gid] = make([]*raft.Raft, len(servers))

		for i, srv := range servers {
			pers := persist.NewMemoryPersister()
			applyCh := make(chan core.ApplyMsg, 1000)

			// Raft
			r := raft.Make(servers, i, pers, net, applyCh)
			tc.groupRafts[gid][i] = r
			net.Register(srv, "Raft", r)

			// ShardGroup
			sg := shardgrp.New(gid)
			tc.groupServers[gid][i] = sg
			net.Register(srv, "ShardGroup", sg)

			// RSM
			rsmObj := rsm.New(r, sg, -1, applyCh)
			sg.SetRSM(rsmObj)
		}
	}

	// Wait for Raft leaders to be elected in each group
	for _, gid := range gids {
		leaderFound := false
		for r := 0; r < 20; r++ {
			time.Sleep(100 * time.Millisecond)
			for i := 0; i < 3; i++ {
				if _, isLeader := tc.groupRafts[gid][i].GetState(); isLeader {
					leaderFound = true
					break
				}
			}
			if leaderFound {
				break
			}
		}
		if !leaderFound {
			t.Fatalf("no leader elected in group %d", gid)
		}
	}

	return tc
}

func (tc *testCluster) cleanup() {
	for _, rafts := range tc.groupRafts {
		for _, r := range rafts {
			r.Kill()
		}
	}
}

func TestShardKVBasic(t *testing.T) {
	tc := makeTestCluster(t)
	defer tc.cleanup()

	// Initialize config
	groups := map[int][]string{
		101: {"g1-0", "g1-1", "g1-2"},
		102: {"g2-0", "g2-1", "g2-2"},
	}
	err := tc.controller.InitConfig(groups)
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	// Create sharded client
	ck := NewClerk(tc.kvsrvAddr, tc.net)

	// Put some keys
	// Key2Shard maps them deterministically
	k1 := "apple"  // shard = Key2Shard("apple")
	k2 := "banana" // shard = Key2Shard("banana")

	putErr1 := ck.Put(k1, "red", 0)
	if putErr1 != core.OK {
		t.Fatalf("Put k1 failed: %v", putErr1)
	}

	putErr2 := ck.Put(k2, "yellow", 0)
	if putErr2 != core.OK {
		t.Fatalf("Put k2 failed: %v", putErr2)
	}

	// Get keys
	val1, ver1, getErr1 := ck.Get(k1)
	if getErr1 != core.OK || val1 != "red" || ver1 != 1 {
		t.Fatalf("Get k1: val=%s ver=%d err=%v", val1, ver1, getErr1)
	}

	val2, ver2, getErr2 := ck.Get(k2)
	if getErr2 != core.OK || val2 != "yellow" || ver2 != 1 {
		t.Fatalf("Get k2: val=%s ver=%d err=%v", val2, ver2, getErr2)
	}

	// Test versioned update
	putErr3 := ck.Put(k1, "green", 1)
	if putErr3 != core.OK {
		t.Fatalf("Put k1 update failed: %v", putErr3)
	}

	val3, ver3, getErr3 := ck.Get(k1)
	if getErr3 != core.OK || val3 != "green" || ver3 != 2 {
		t.Fatalf("Get k1 updated: val=%s ver=%d err=%v", val3, ver3, getErr3)
	}
}

func TestShardMigration(t *testing.T) {
	tc := makeTestCluster(t)
	defer tc.cleanup()

	// Initial configuration with only Group 101
	groups := map[int][]string{
		101: {"g1-0", "g1-1", "g1-2"},
	}
	err := tc.controller.InitConfig(groups)
	if err != nil {
		t.Fatalf("InitConfig failed: %v", err)
	}

	ck := NewClerk(tc.kvsrvAddr, tc.net)

	// Write key to Group 101 (owns all shards)
	k1 := "grape"
	putErr := ck.Put(k1, "purple", 0)
	if putErr != core.OK {
		t.Fatalf("Put failed: %v", putErr)
	}

	// Check it was stored
	val, ver, getErr := ck.Get(k1)
	if getErr != core.OK || val != "purple" || ver != 1 {
		t.Fatalf("Get failed: val=%s ver=%d err=%v", val, ver, getErr)
	}

	// Now define target config where Group 102 is added and owns some shards
	shard := core.Key2Shard(k1)
	target := core.ShardConfig{
		Num: 1,
		Groups: map[int][]string{
			101: {"g1-0", "g1-1", "g1-2"},
			102: {"g2-0", "g2-1", "g2-2"},
		},
	}
	// Let Group 102 own the shard of k1
	for i := 0; i < core.NShards; i++ {
		if i == shard {
			target.Shards[i] = 102
		} else {
			target.Shards[i] = 101
		}
	}

	// Trigger configuration change / shard migration!
	migErr := tc.controller.ChangeConfigTo(target)
	if migErr != nil {
		t.Fatalf("ChangeConfigTo failed: %v", migErr)
	}

	// Wait briefly for migration completion/polling
	time.Sleep(300 * time.Millisecond)

	// Query key via sharded client: should query new config, find new owner (Group 102),
	// route to Group 102, and retrieve migrated value and version!
	val2, ver2, getErr2 := ck.Get(k1)
	if getErr2 != core.OK || val2 != "purple" || ver2 != 1 {
		t.Fatalf("Get migrated key failed: val=%s ver=%d err=%v", val2, ver2, getErr2)
	}

	// Update key on the new owner
	putErr2 := ck.Put(k1, "white", 1)
	if putErr2 != core.OK {
		t.Fatalf("Put on migrated shard failed: %v", putErr2)
	}

	val3, ver3, getErr3 := ck.Get(k1)
	if getErr3 != core.OK || val3 != "white" || ver3 != 2 {
		t.Fatalf("Get after update on migrated shard failed: val=%s ver=%d err=%v", val3, ver3, getErr3)
	}
}
