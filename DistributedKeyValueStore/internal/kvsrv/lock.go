package kvsrv

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"dkv/internal/core"
)

// Lock implements a distributed lock using the KV server's versioned puts.
//
// Acquire spins on Put(lockKey, clientID, currentVersion) until it succeeds.
// Release calls Put(lockKey, "", currentVersion) to clear the lock.
//
// This is the coordination primitive used by the ShardController for
// atomic configuration updates when multiple controllers are running (5C).
type Lock struct {
	ck       *Clerk
	lockName string
	clientID string
}

// NewLock creates a Lock backed by the given Clerk.
func NewLock(ck *Clerk, lockName string) *Lock {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	clientID := hex.EncodeToString(b)
	return &Lock{
		ck:       ck,
		lockName: lockName,
		clientID: clientID,
	}
}

// Acquire blocks until the lock is successfully held.
func (l *Lock) Acquire() {
	for {
		val, version, err := l.ck.Get(l.lockName)
		if err == core.ErrNoKey {
			putErr := l.ck.Put(l.lockName, l.clientID, 0)
			if putErr == core.OK || putErr == core.ErrMaybe {
				v, _, e := l.ck.Get(l.lockName)
				if e == core.OK && v == l.clientID {
					return
				}
			}
		} else if err == core.OK {
			if val == l.clientID {
				return
			}
			if val == "" {
				putErr := l.ck.Put(l.lockName, l.clientID, version)
				if putErr == core.OK || putErr == core.ErrMaybe {
					v, _, e := l.ck.Get(l.lockName)
					if e == core.OK && v == l.clientID {
						return
					}
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Release relinquishes the lock.
func (l *Lock) Release() {
	for {
		val, version, err := l.ck.Get(l.lockName)
		if err == core.OK {
			if val != l.clientID {
				return
			}
			putErr := l.ck.Put(l.lockName, "", version)
			if putErr == core.OK || putErr == core.ErrMaybe {
				v, _, e := l.ck.Get(l.lockName)
				if e == core.ErrNoKey || (e == core.OK && v == "") {
					return
				}
			}
		} else {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
