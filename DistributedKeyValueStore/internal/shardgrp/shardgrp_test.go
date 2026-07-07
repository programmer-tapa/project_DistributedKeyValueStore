package shardgrp

import (
	"testing"

	"dkv/internal/core"
)

func TestShardOwnershipRouting(t *testing.T) {
	sg := New(101)

	// Initially all owned shards are false
	getOp := core.GetArgs{Key: "hello"}
	rep := sg.DoOp(getOp)
	getRep, ok := rep.(core.GetReply)
	if !ok || getRep.Err != core.ErrWrongGroup {
		t.Fatalf("Expected ErrWrongGroup, got rep=%v", rep)
	}

	putOp := core.PutArgs{Key: "hello", Value: "world", Version: 0}
	rep = sg.DoOp(putOp)
	putRep, ok := rep.(core.PutReply)
	if !ok || putRep.Err != core.ErrWrongGroup {
		t.Fatalf("Expected ErrWrongGroup, got rep=%v", rep)
	}
}

func TestShardMigration(t *testing.T) {
	sg := New(101)

	// 1. Install shard 3 at config 1
	installArgs := core.InstallShardArgs{
		Shard:     3,
		ConfigNum: 1,
		Data: map[string]core.VersionedValue{
			"grape": {Value: "world", Version: 1},
		},
	}
	rep := sg.DoOp(installArgs)
	installRep, ok := rep.(core.InstallShardReply)
	if !ok || installRep.Err != core.OK {
		t.Fatalf("Install failed: rep=%v", rep)
	}

	// 2. Put key "grape" (update version 1 -> 2)
	putArgs := core.PutArgs{Key: "grape", Value: "universe", Version: 1}
	rep = sg.DoOp(putArgs)
	putRep, ok := rep.(core.PutReply)
	if !ok || putRep.Err != core.OK {
		t.Fatalf("Put failed: rep=%v", rep)
	}

	// 3. Freeze shard 3 at config 2
	freezeArgs := core.FreezeShardArgs{
		Shard:     3,
		ConfigNum: 2,
	}
	rep = sg.DoOp(freezeArgs)
	freezeRep, ok := rep.(core.FreezeShardReply)
	if !ok || freezeRep.Err != core.OK {
		t.Fatalf("Freeze failed: rep=%v", rep)
	}
	val, ok := freezeRep.Data["grape"]
	if !ok || val.Value != "universe" || val.Version != 2 {
		t.Fatalf("Incorrect frozen data: %v", freezeRep.Data)
	}

	// 4. Delete shard 3 at config 2
	deleteArgs := core.DeleteShardArgs{
		Shard:     3,
		ConfigNum: 2,
	}
	rep = sg.DoOp(deleteArgs)
	deleteRep, ok := rep.(core.DeleteShardReply)
	if !ok || deleteRep.Err != core.OK {
		t.Fatalf("Delete failed: rep=%v", rep)
	}

	// 5. Verify it's no longer owned
	getArgs := core.GetArgs{Key: "grape"}
	rep = sg.DoOp(getArgs)
	getRep, ok := rep.(core.GetReply)
	if !ok || getRep.Err != core.ErrWrongGroup {
		t.Fatalf("Expected ErrWrongGroup after delete, got %v", rep)
	}
}

func TestConfigNumFencing(t *testing.T) {
	sg := New(101)

	// 1. Install shard 5 at config 10
	installArgs := core.InstallShardArgs{
		Shard:     5,
		ConfigNum: 10,
		Data:      nil,
	}
	sg.DoOp(installArgs)

	// 2. Try to freeze shard 5 with stale config 9
	freezeArgs := core.FreezeShardArgs{
		Shard:     5,
		ConfigNum: 9,
	}
	rep := sg.DoOp(freezeArgs)
	freezeRep, ok := rep.(core.FreezeShardReply)
	if !ok || freezeRep.Err != core.ErrVersion {
		t.Fatalf("Expected ErrVersion for stale config, got %v", rep)
	}

	// 3. Try to install shard 5 with stale config 8
	installStale := core.InstallShardArgs{
		Shard:     5,
		ConfigNum: 8,
		Data:      nil,
	}
	rep = sg.DoOp(installStale)
	installRep, ok := rep.(core.InstallShardReply)
	if !ok || installRep.Err != core.ErrVersion {
		t.Fatalf("Expected ErrVersion for stale install, got %v", rep)
	}
}
