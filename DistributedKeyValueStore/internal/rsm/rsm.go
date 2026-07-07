// Package rsm implements the Replicated State Machine abstraction.
package rsm

import (
	"encoding/gob"
	"errors"
	"math/rand"
	"sync"
	"time"

	"dkv/internal/core"
	"dkv/internal/raft"
)

func init() {
	gob.Register(core.Op{})
}

type SubResult struct {
	id  int64
	rep interface{}
}

// RSM wraps a Raft peer and a StateMachine to provide replicated execution.
type RSM struct {
	mu           sync.Mutex
	rf           *raft.Raft
	sm           core.StateMachine
	maxraftstate int
	applyCh      chan core.ApplyMsg
	lastApplied  int
	notify       map[int]chan SubResult
}

// New creates an RSM wrapping the given Raft instance and state machine.
// maxraftstate: trigger snapshot when PersistBytes() exceeds this (-1 = never snapshot).
func New(rf *raft.Raft, sm core.StateMachine, maxraftstate int, applyCh chan core.ApplyMsg) *RSM {
	rsm := &RSM{
		rf:           rf,
		sm:           sm,
		maxraftstate: maxraftstate,
		applyCh:      applyCh,
		notify:       make(map[int]chan SubResult),
	}

	go rsm.reader()

	return rsm
}

// Submit proposes an operation through Raft. Blocks until the operation is
// committed and applied, or returns an error if leadership is lost.
func (r *RSM) Submit(op interface{}) (interface{}, error) {
	if r.rf == nil {
		return nil, errors.New("rsm: no raft peer")
	}

	opWrapper := core.Op{
		ID:      rand.Int63(),
		Payload: op,
	}

	index, term, isLeader := r.rf.Start(opWrapper)
	if !isLeader {
		return nil, errors.New("rsm: not leader")
	}

	r.mu.Lock()
	ch := make(chan SubResult, 1)
	r.notify[index] = ch
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.notify, index)
		r.mu.Unlock()
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case res := <-ch:
			if res.id == opWrapper.ID {
				return res.rep, nil
			}
			return nil, errors.New("rsm: leadership lost during replication")
		case <-ticker.C:
			currentTerm, isLeader := r.rf.GetState()
			if !isLeader || currentTerm != term {
				return nil, errors.New("rsm: leadership lost")
			}
		}
	}
}

func (r *RSM) reader() {
	for msg := range r.applyCh {
		if msg.CommandValid {
			op, ok := msg.Command.(core.Op)
			if !ok {
				continue
			}

			rep := r.sm.DoOp(op.Payload)

			r.mu.Lock()
			r.lastApplied = msg.CommandIndex

			if ch, ok := r.notify[msg.CommandIndex]; ok {
				select {
				case ch <- SubResult{
					id:  op.ID,
					rep: rep,
				}:
				default:
				}
			}

			if r.maxraftstate != -1 && r.rf.PersistBytes() >= r.maxraftstate {
				snapshot := r.sm.Snapshot()
				r.rf.Snapshot(msg.CommandIndex, snapshot)
			}
			r.mu.Unlock()

		} else if msg.SnapshotValid {
			r.mu.Lock()
			if msg.SnapshotIndex > r.lastApplied {
				r.sm.Restore(msg.Snapshot)
				r.lastApplied = msg.SnapshotIndex

				for idx, ch := range r.notify {
					if idx <= msg.SnapshotIndex {
						select {
						case ch <- SubResult{
							id:  -1,
							rep: nil,
						}:
						default:
						}
					}
				}
			}
			r.mu.Unlock()
		}
	}

	r.mu.Lock()
	for _, ch := range r.notify {
		select {
		case ch <- SubResult{
			id:  -1,
			rep: nil,
		}:
		default:
		}
	}
	r.mu.Unlock()
}
