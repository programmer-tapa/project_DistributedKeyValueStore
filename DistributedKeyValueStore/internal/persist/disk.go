package persist

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// DiskPersister implements core.Persister using file-based storage.
type DiskPersister struct {
	mu  sync.Mutex
	dir string
}

// NewDiskPersister creates a file-based persister rooted at the given directory.
func NewDiskPersister(dir string) *DiskPersister {
	_ = os.MkdirAll(dir, 0755)
	return &DiskPersister{dir: dir}
}

func (p *DiskPersister) writeAtomic(filename string, data []byte) {
	tmpPath := filename + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return
	}
	if err := f.Sync(); err != nil {
		return
	}
	f.Close()
	_ = os.Rename(tmpPath, filename)
}

// Save atomically persists Raft state and optional snapshot.
func (p *DiskPersister) Save(raftstate []byte, snapshot []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	statePath := filepath.Join(p.dir, "raftstate.bin")
	p.writeAtomic(statePath, raftstate)

	snapPath := filepath.Join(p.dir, "snapshot.bin")
	if len(snapshot) > 0 {
		p.writeAtomic(snapPath, snapshot)
	} else {
		_ = os.Remove(snapPath)
	}
}

// ReadRaftState returns the most recently persisted Raft state.
func (p *DiskPersister) ReadRaftState() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()

	statePath := filepath.Join(p.dir, "raftstate.bin")
	data, err := ioutil.ReadFile(statePath)
	if err != nil {
		return nil
	}
	return data
}

// ReadSnapshot returns the most recently persisted snapshot.
func (p *DiskPersister) ReadSnapshot() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()

	snapPath := filepath.Join(p.dir, "snapshot.bin")
	data, err := ioutil.ReadFile(snapPath)
	if err != nil {
		return nil
	}
	return data
}

// PersistBytes returns the size of the persisted Raft state.
func (p *DiskPersister) PersistBytes() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	statePath := filepath.Join(p.dir, "raftstate.bin")
	info, err := os.Stat(statePath)
	if err != nil {
		return 0
	}
	return int(info.Size())
}
