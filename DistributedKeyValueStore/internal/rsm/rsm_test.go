package rsm

import (
	"testing"
	"time"

	"dkv/internal/core"
	"dkv/internal/kvsrv"
	"dkv/internal/persist"
	"dkv/internal/raft"
	"dkv/internal/transport"
)

func TestRSMBasic(t *testing.T) {
	net := transport.NewLocalNetwork()
	peers := []string{"node0", "node1", "node2"}

	rafts := make([]*raft.Raft, 3)
	rsms := make([]*RSM, 3)
	kvs := make([]*kvsrv.KVServer, 3)

	for i := 0; i < 3; i++ {
		pers := persist.NewMemoryPersister()
		applyCh := make(chan core.ApplyMsg, 1000)
		rafts[i] = raft.Make(peers, i, pers, net, applyCh)
		net.Register(peers[i], "Raft", rafts[i])

		kvs[i] = kvsrv.New()
		rsms[i] = New(rafts[i], kvs[i], -1, applyCh)
	}

	defer func() {
		for i := 0; i < 3; i++ {
			rafts[i].Kill()
		}
	}()

	// Wait for leader
	var leaderIdx int
	leaderFound := false
	for r := 0; r < 20; r++ {
		time.Sleep(150 * time.Millisecond)
		for i := 0; i < 3; i++ {
			if _, isLeader := rafts[i].GetState(); isLeader {
				leaderIdx = i
				leaderFound = true
				break
			}
		}
		if leaderFound {
			break
		}
	}

	if !leaderFound {
		t.Fatalf("no leader elected")
	}

	// Submit operation
	op := core.PutArgs{Key: "hello", Value: "world", Version: 0}
	rep, err := rsms[leaderIdx].Submit(op)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	putRep, ok := rep.(core.PutReply)
	if !ok || putRep.Err != core.OK {
		t.Fatalf("Unexpected reply: %v", rep)
	}

	// Check replica stores
	time.Sleep(200 * time.Millisecond)
	for i := 0; i < 3; i++ {
		var getRep core.GetReply
		_ = kvs[i].Get(&core.GetArgs{Key: "hello"}, &getRep)
		if getRep.Err != core.OK || getRep.Value != "world" {
			t.Errorf("Replica %d did not apply operation: err=%v val=%s", i, getRep.Err, getRep.Value)
		}
	}
}
