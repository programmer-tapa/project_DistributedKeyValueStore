# Decoding `raftstate.bin`

The `raftstate.bin` file contains the serialized persistent state of a Raft node, which includes the current term, who the node voted for, and the replicated log entries. 

Because the project utilizes Go's native **`encoding/gob`** package for serialization, the file is in binary format and cannot be parsed using standard text-editing utilities (such as `cat` or `strings`). 

This guide explains how to decode and inspect the contents of `raftstate.bin`.

---

## 1. The Decoding Utility

Save the following Go script to your Go module root (`DistributedKeyValueStore/decode_raftstate.go`). It registers the various command and reply payload structures and deserializes the state file.

```go
package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"

	"dkv/internal/core"
)

func init() {
	// Register all DKV/Raft payload types so the GOB decoder recognizes them
	gob.Register(core.RequestVoteArgs{})
	gob.Register(core.RequestVoteReply{})
	gob.Register(core.AppendEntriesArgs{})
	gob.Register(core.AppendEntriesReply{})
	gob.Register(core.InstallSnapshotArgs{})
	gob.Register(core.InstallSnapshotReply{})
	gob.Register(core.GetArgs{})
	gob.Register(core.PutArgs{})
	gob.Register(core.FreezeShardArgs{})
	gob.Register(core.InstallShardArgs{})
	gob.Register(core.DeleteShardArgs{})
	gob.Register(core.VersionedValue{})
	gob.Register(core.GetReply{})
	gob.Register(core.PutReply{})
	gob.Register(core.FreezeShardReply{})
	gob.Register(core.InstallShardReply{})
	gob.Register(core.DeleteShardReply{})
	gob.Register(core.Op{})
}

func main() {
	// Reads the raftstate.bin file from the level above
	data, err := ioutil.ReadFile("../raftstate.bin")
	if err != nil {
		log.Fatalf("failed to read file: %v", err)
	}

	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)

	var currentTerm int
	var votedFor int
	var logEntries []core.LogEntry
	var lastIncludedIndex int
	var lastIncludedTerm int

	// Decode elements in the exact sequence they were encoded
	if err := d.Decode(&currentTerm); err != nil {
		log.Fatalf("decode currentTerm error: %v", err)
	}
	if err := d.Decode(&votedFor); err != nil {
		log.Fatalf("decode votedFor error: %v", err)
	}
	if err := d.Decode(&logEntries); err != nil {
		log.Fatalf("decode logEntries error: %v", err)
	}
	if err := d.Decode(&lastIncludedIndex); err != nil {
		log.Fatalf("decode lastIncludedIndex error: %v", err)
	}
	if err := d.Decode(&lastIncludedTerm); err != nil {
		log.Fatalf("decode lastIncludedTerm error: %v", err)
	}

	fmt.Println("--- Raft State ---")
	fmt.Printf("Current Term:        %d\n", currentTerm)
	fmt.Printf("Voted For:           %d\n", votedFor)
	fmt.Printf("Last Included Index: %d\n", lastIncludedIndex)
	fmt.Printf("Last Included Term:  %d\n", lastIncludedTerm)
	
	fmt.Println("\n--- Log Entries ---")
	for _, entry := range logEntries {
		fmt.Printf("[%d] Term: %d, Command: %+v\n", entry.Index, entry.Term, entry.Command)
	}
}
```

---

## 2. Running the Utility

1. Copy the `raftstate.bin` file from a container (e.g., `dkv-kvraft-1`):
   ```bash
   docker cp dkv-kvraft-1:/var/lib/raft/raftstate.bin ./raftstate.bin
   ```
2. Navigate into the Go module directory:
   ```bash
   cd DistributedKeyValueStore/
   ```
3. Run the script:
   ```bash
   go run decode_raftstate.go
   ```

---

## 3. Example Output

Executing this utility prints a structured visualization of the binary state:

```text
--- Raft State ---
Current Term:        17
Voted For:           0
Last Included Index: 0
Last Included Term:  0

--- Log Entries ---
[0] Term: 0, Command: <nil>
[1] Term: 1, Command: {ID:1344302399593275084 Payload:{Shard:0 ConfigNum:0 Data:map[]}}
[2] Term: 1, Command: {ID:4864341903405136959 Payload:{Shard:2 ConfigNum:0 Data:map[]}}
[3] Term: 1, Command: {ID:746312628131951267 Payload:{Shard:4 ConfigNum:0 Data:map[]}}
...
[16] Term: 5, Command: {ID:8274337161472779569 Payload:{Key:non_existent_key}}
...
[55] Term: 17, Command: {ID:843436505486485012 Payload:{Shard:8 ConfigNum:0 Data:map[]}}
```
