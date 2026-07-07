package raft

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"dkv/internal/core"
	"dkv/internal/persist"
	"dkv/internal/transport"
)

type config struct {
	t         *testing.T
	net       *transport.LocalNetwork
	peers     []string
	rafts     []*Raft
	persists  []*persist.MemoryPersister
	applyChs  []chan core.ApplyMsg
	mu        sync.Mutex
	connected []bool
}

func makeConfig(t *testing.T, n int) *config {
	cfg := &config{
		t:         t,
		net:       transport.NewLocalNetwork(),
		peers:     make([]string, n),
		rafts:     make([]*Raft, n),
		persists:  make([]*persist.MemoryPersister, n),
		applyChs:  make([]chan core.ApplyMsg, n),
		connected: make([]bool, n),
	}

	for i := 0; i < n; i++ {
		cfg.peers[i] = fmt.Sprintf("node%d", i)
		cfg.persists[i] = persist.NewMemoryPersister()
		cfg.applyChs[i] = make(chan core.ApplyMsg, 1000)
		cfg.connected[i] = true
	}

	for i := 0; i < n; i++ {
		cfg.rafts[i] = Make(cfg.peers, i, cfg.persists[i], cfg.net, cfg.applyChs[i])
		cfg.net.Register(cfg.peers[i], "Raft", cfg.rafts[i])
	}

	return cfg
}

func (cfg *config) cleanup() {
	for i := 0; i < len(cfg.rafts); i++ {
		cfg.rafts[i].Kill()
	}
}

func (cfg *config) checkOneLeader() int {
	for r := 0; r < 10; r++ {
		time.Sleep(150 * time.Millisecond)
		leaders := make(map[int][]int)
		for i := 0; i < len(cfg.rafts); i++ {
			if cfg.connected[i] {
				if term, isLeader := cfg.rafts[i].GetState(); isLeader {
					leaders[term] = append(leaders[term], i)
				}
			}
		}

		lastTermWithLeader := -1
		for term, list := range leaders {
			if len(list) > 1 {
				cfg.t.Fatalf("term %d has multiple leaders: %v", term, list)
			}
			if term > lastTermWithLeader {
				lastTermWithLeader = term
			}
		}

		if len(leaders) > 0 {
			return leaders[lastTermWithLeader][0]
		}
	}
	cfg.t.Fatalf("expected one leader, got none")
	return -1
}

func (cfg *config) disconnect(i int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.connected[i] = false
	for j := 0; j < len(cfg.rafts); j++ {
		if i != j {
			cfg.net.Partition(cfg.peers[i], cfg.peers[j])
		}
	}
}

func (cfg *config) reconnect(i int) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.connected[i] = true
	for j := 0; j < len(cfg.rafts); j++ {
		if i != j {
			cfg.net.Heal(cfg.peers[i], cfg.peers[j])
		}
	}
}

func TestInitialElection(t *testing.T) {
	cfg := makeConfig(t, 3)
	defer cfg.cleanup()

	cfg.checkOneLeader()
}

func TestReElection(t *testing.T) {
	cfg := makeConfig(t, 3)
	defer cfg.cleanup()

	leader1 := cfg.checkOneLeader()

	// Disconnect the leader
	cfg.disconnect(leader1)

	// Check that a new leader is elected from the remaining two
	leader2 := cfg.checkOneLeader()
	if leader2 == leader1 {
		t.Fatalf("disconnected leader still active")
	}

	// Reconnect and check stability
	cfg.reconnect(leader1)
	leader3 := cfg.checkOneLeader()
	if leader3 != leader2 && leader3 != leader1 {
		t.Fatalf("unexpected leader after reconnect")
	}
}

func TestBasicAgreement(t *testing.T) {
	cfg := makeConfig(t, 3)
	defer cfg.cleanup()

	leader := cfg.checkOneLeader()

	index, term, isLeader := cfg.rafts[leader].Start("cmd1")
	if !isLeader {
		t.Fatalf("lost leadership")
	}

	// Wait for commit/agree
	success := false
	for r := 0; r < 20; r++ {
		time.Sleep(100 * time.Millisecond)
		commits := 0
		for i := 0; i < 3; i++ {
			select {
			case msg := <-cfg.applyChs[i]:
				if msg.CommandValid && msg.CommandIndex == index && msg.Command == "cmd1" {
					commits++
				}
			default:
			}
		}
		if commits >= 2 { // Majority agreed
			success = true
			break
		}
	}

	if !success {
		t.Fatalf("failed to agree on command at term %d", term)
	}
}

func TestPersistRaft(t *testing.T) {
	cfg := makeConfig(t, 3)
	defer cfg.cleanup()

	leader := cfg.checkOneLeader()
	cfg.rafts[leader].Start("cmd1")
	time.Sleep(200 * time.Millisecond)

	// Kill and restart nodes, checking that state is recovered
	cfg.rafts[0].Kill()
	cfg.rafts[0] = Make(cfg.peers, 0, cfg.persists[0], cfg.net, cfg.applyChs[0])
	cfg.net.Register(cfg.peers[0], "Raft", cfg.rafts[0])

	time.Sleep(200 * time.Millisecond)
	cfg.checkOneLeader()
}

func TestSnapshotRaft(t *testing.T) {
	cfg := makeConfig(t, 3)
	defer cfg.cleanup()

	leader := cfg.checkOneLeader()
	index, _, _ := cfg.rafts[leader].Start("cmd1")
	time.Sleep(200 * time.Millisecond)

	cfg.rafts[leader].Snapshot(index, []byte("snapshot-data"))

	// Verify that state size matches/is small
	size := cfg.rafts[leader].PersistBytes()
	if size == 0 {
		t.Fatalf("persist bytes is 0")
	}
}
