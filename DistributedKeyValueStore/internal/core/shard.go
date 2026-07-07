package core

// NShards is the total number of shards in the system.
// Keys are mapped to shards via Key2Shard().
const NShards = 10

// ShardConfig describes the current shard-to-group mapping.
// Configuration changes are monotonically numbered.
type ShardConfig struct {
	Num    int              // Monotonically increasing config number
	Shards [NShards]int    // Shards[i] = GID that owns shard i (0 = unassigned)
	Groups map[int][]string // GID → list of server addresses in that group
}

// Key2Shard maps a key to its shard number (deterministic hash).
func Key2Shard(key string) int {
	shard := 0
	if len(key) > 0 {
		shard = int(key[0])
	}
	shard %= NShards
	return shard
}

// --- Shard Migration RPC Types ---

// FreezeShardArgs tells a shard group to stop serving a shard and return its data.
type FreezeShardArgs struct {
	Shard     int // Which shard to freeze
	ConfigNum int // Must be > group's last seen configNum for this shard
}

// FreezeShardReply contains the frozen shard's KV data.
type FreezeShardReply struct {
	Err  Err
	Data map[string]VersionedValue // The shard's key-value pairs
}

// VersionedValue pairs a value with its version for shard transfer.
type VersionedValue struct {
	Value   string
	Version uint64
}

// InstallShardArgs sends shard data to the destination group.
type InstallShardArgs struct {
	Shard     int
	ConfigNum int
	Data      map[string]VersionedValue
}

// InstallShardReply is the response to InstallShard.
type InstallShardReply struct {
	Err Err
}

// DeleteShardArgs tells the source group to discard a migrated shard.
type DeleteShardArgs struct {
	Shard     int
	ConfigNum int
}

// DeleteShardReply is the response to DeleteShard.
type DeleteShardReply struct {
	Err Err
}
