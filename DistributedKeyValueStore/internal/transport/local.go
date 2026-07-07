package transport

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"time"
)

// LocalNetwork implements core.Network using in-memory channels.
// Simulates network behavior for unit testing.
type LocalNetwork struct {
	mu         sync.Mutex
	receivers  map[string]map[string]reflect.Value // peerAddr -> serviceName -> receiver Value
	disabled   map[string]map[string]bool          // peer1 -> peer2 -> partitioned
	unreliable bool
}

// NewLocalNetwork creates an in-memory network for testing.
func NewLocalNetwork() *LocalNetwork {
	return &LocalNetwork{
		receivers: make(map[string]map[string]reflect.Value),
		disabled:  make(map[string]map[string]bool),
	}
}

// Register registers a service receiver at a peer address.
func (ln *LocalNetwork) Register(peerAddr string, serviceName string, rcvr interface{}) {
	ln.mu.Lock()
	defer ln.mu.Unlock()
	if _, ok := ln.receivers[peerAddr]; !ok {
		ln.receivers[peerAddr] = make(map[string]reflect.Value)
	}
	ln.receivers[peerAddr][serviceName] = reflect.ValueOf(rcvr)
}

// Call sends an RPC to the peer via in-memory channels.
// Returns false to simulate message loss or partitions.
func (ln *LocalNetwork) Call(peerAddr string, method string, args interface{}, reply interface{}) bool {
	parts := strings.Split(method, ".")
	if len(parts) != 2 {
		return false
	}
	serviceName, methodName := parts[0], parts[1]

	ln.mu.Lock()
	if ln.unreliable && rand.Intn(100) < 15 {
		ln.mu.Unlock()
		time.Sleep(time.Duration(rand.Intn(15)) * time.Millisecond)
		return false
	}

	// Resolve caller address
	callerAddr := ln.getCallerAddr(peerAddr, args)
	if callerAddr != "" {
		if m, ok := ln.disabled[callerAddr]; ok && m[peerAddr] {
			ln.mu.Unlock()
			time.Sleep(100 * time.Millisecond) // Simulate timeout
			return false
		}
	}

	srvs, ok := ln.receivers[peerAddr]
	if !ok {
		ln.mu.Unlock()
		return false
	}
	rcvr, ok := srvs[serviceName]
	if !ok {
		ln.mu.Unlock()
		return false
	}
	ln.mu.Unlock()

	m := rcvr.MethodByName(methodName)
	if !m.IsValid() {
		return false
	}

	in := []reflect.Value{reflect.ValueOf(args), reflect.ValueOf(reply)}
	m.Call(in)
	return true
}

// Partition disconnects two peers from each other (bidirectional).
func (ln *LocalNetwork) Partition(peer1, peer2 string) {
	ln.mu.Lock()
	defer ln.mu.Unlock()
	if _, ok := ln.disabled[peer1]; !ok {
		ln.disabled[peer1] = make(map[string]bool)
	}
	if _, ok := ln.disabled[peer2]; !ok {
		ln.disabled[peer2] = make(map[string]bool)
	}
	ln.disabled[peer1][peer2] = true
	ln.disabled[peer2][peer1] = true
}

// Heal restores connectivity between two peers.
func (ln *LocalNetwork) Heal(peer1, peer2 string) {
	ln.mu.Lock()
	defer ln.mu.Unlock()
	if ln.disabled[peer1] != nil {
		delete(ln.disabled[peer1], peer2)
	}
	if ln.disabled[peer2] != nil {
		delete(ln.disabled[peer2], peer1)
	}
}

// SetUnreliable enables random message drops for chaos testing.
func (ln *LocalNetwork) SetUnreliable(unreliable bool) {
	ln.mu.Lock()
	defer ln.mu.Unlock()
	ln.unreliable = unreliable
}

// getCallerAddr infers the sender address by extracting LeaderID/CandidateID/Me index
// and prefixing it with the destination's non-numeric prefix.
func (ln *LocalNetwork) getCallerAddr(destAddr string, args interface{}) string {
	val := reflect.ValueOf(args)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return ""
	}
	idx := -1
	for _, name := range []string{"CandidateID", "LeaderID", "Me"} {
		f := val.FieldByName(name)
		if f.IsValid() {
			if f.Kind() == reflect.Int {
				idx = int(f.Int())
				break
			}
		}
	}
	if idx == -1 {
		return ""
	}

	i := len(destAddr) - 1
	for i >= 0 && destAddr[i] >= '0' && destAddr[i] <= '9' {
		i--
	}
	prefix := destAddr[:i+1]
	return fmt.Sprintf("%s%d", prefix, idx)
}
