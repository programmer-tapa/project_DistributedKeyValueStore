// Package persist provides concrete implementations of core.Persister.
package persist

// MemoryPersister implements core.Persister using in-memory byte slices.
// Used for unit tests (mirrors the lab's tester1/persister.go).
type MemoryPersister struct {
	raftstate []byte
	snapshot  []byte
}

// NewMemoryPersister creates an empty in-memory persister.
func NewMemoryPersister() *MemoryPersister {
	return &MemoryPersister{}
}

func (p *MemoryPersister) Save(raftstate []byte, snapshot []byte) {
	p.raftstate = make([]byte, len(raftstate))
	copy(p.raftstate, raftstate)
	if snapshot != nil {
		p.snapshot = make([]byte, len(snapshot))
		copy(p.snapshot, snapshot)
	}
}

func (p *MemoryPersister) ReadRaftState() []byte { return p.raftstate }
func (p *MemoryPersister) ReadSnapshot() []byte  { return p.snapshot }
func (p *MemoryPersister) PersistBytes() int      { return len(p.raftstate) }
