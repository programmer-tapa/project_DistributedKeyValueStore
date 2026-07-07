package kvsrv

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"dkv/internal/core"
	"dkv/internal/transport"
)

func TestReliablePut(t *testing.T) {
	net := transport.NewLocalNetwork()
	server := New()
	net.Register("server", "KVServer", server)

	clerk := NewClerk("server", net)

	// Put new key
	err := clerk.Put("a", "1", 0)
	if err != core.OK {
		t.Fatalf("Put failed: %v", err)
	}

	// Get key
	val, version, err := clerk.Get("a")
	if err != core.OK || val != "1" || version != 1 {
		t.Fatalf("Get failed: val=%s version=%d err=%v", val, version, err)
	}

	// Put replacement
	err = clerk.Put("a", "2", 1)
	if err != core.OK {
		t.Fatalf("Put replacement failed: %v", err)
	}

	val, version, err = clerk.Get("a")
	if err != core.OK || val != "2" || version != 2 {
		t.Fatalf("Get failed: val=%s version=%d err=%v", val, version, err)
	}

	// Put with bad version
	err = clerk.Put("a", "3", 1)
	if err != core.ErrVersion {
		t.Fatalf("Expected ErrVersion, got %v", err)
	}
}

func TestPutConcurrentReliable(t *testing.T) {
	net := transport.NewLocalNetwork()
	server := New()
	net.Register("server", "KVServer", server)

	const nClients = 10
	const nOps = 20
	var wg sync.WaitGroup

	for i := 0; i < nClients; i++ {
		wg.Add(1)
		go func(cid int) {
			defer wg.Done()
			clerk := NewClerk("server", net)
			key := fmt.Sprintf("k-%d", cid)
			for j := 0; j < nOps; j++ {
				_, version, err := clerk.Get(key)
				if err != core.OK && err != core.ErrNoKey {
					t.Errorf("Get err: %v", err)
					return
				}
				putErr := clerk.Put(key, fmt.Sprintf("val-%d", j), version)
				if putErr != core.OK {
					t.Errorf("Put err: %v", putErr)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestUnreliableNet(t *testing.T) {
	net := transport.NewLocalNetwork()
	server := New()
	net.Register("server", "KVServer", server)
	net.SetUnreliable(true)

	clerk := NewClerk("server", net)

	// Keep trying to Put
	err := clerk.Put("k", "v1", 0)
	if err != core.OK && err != core.ErrMaybe {
		t.Fatalf("Put failed: %v", err)
	}

	val, _, err := clerk.Get("k")
	if err != core.OK || val != "v1" {
		t.Fatalf("Get k failed: val=%s, err=%v", val, err)
	}
}

func TestLockBasic(t *testing.T) {
	net := transport.NewLocalNetwork()
	server := New()
	net.Register("server", "KVServer", server)

	clerk := NewClerk("server", net)
	lock := NewLock(clerk, "mylock")

	lock.Acquire()
	lock.Release()
}

func TestLockManyClients(t *testing.T) {
	net := transport.NewLocalNetwork()
	server := New()
	net.Register("server", "KVServer", server)

	const nClients = 5
	var wg sync.WaitGroup
	counter := 0

	for i := 0; i < nClients; i++ {
		wg.Add(1)
		go func(cid int) {
			defer wg.Done()
			clerk := NewClerk("server", net)
			lock := NewLock(clerk, "mylock")

			for j := 0; j < 5; j++ {
				lock.Acquire()
				curr := counter
				time.Sleep(10 * time.Millisecond)
				counter = curr + 1
				lock.Release()
			}
		}(i)
	}
	wg.Wait()

	clerk := NewClerk("server", net)
	val, _, err := clerk.Get("mylock")
	if err == core.OK && val != "" {
		t.Fatalf("Lock was not released properly: %s", val)
	}

	if counter != nClients*5 {
		t.Fatalf("Counter mismatch: expected %d, got %d", nClients*5, counter)
	}
}
