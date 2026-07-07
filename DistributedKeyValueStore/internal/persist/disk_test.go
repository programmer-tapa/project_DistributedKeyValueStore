package persist

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestDiskPersister(t *testing.T) {
	dir, err := ioutil.TempDir("", "disk_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	p := NewDiskPersister(dir)

	if p.ReadRaftState() != nil {
		t.Fatalf("expected nil state")
	}

	state := []byte("raft-state-data")
	snap := []byte("snapshot-data")

	p.Save(state, snap)

	if string(p.ReadRaftState()) != "raft-state-data" {
		t.Fatalf("incorrect read state")
	}

	if string(p.ReadSnapshot()) != "snapshot-data" {
		t.Fatalf("incorrect read snapshot")
	}

	if p.PersistBytes() != len(state) {
		t.Fatalf("incorrect persist bytes size")
	}
}
