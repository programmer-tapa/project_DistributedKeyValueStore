# dkv-client

`dkv-client` is the CLI client application for interacting with the distributed sharded key-value store. It translates high-level command line instructions (`get`, `put`) into RPC requests, routes them to the correct server node, and prints the response.

---

## Capabilities

1. **Shard Routing:** Connects to the central `ShardController` to query the latest configuration, finding which replica group owns a target key.
2. **Version-Guarded Puts:** Supports version checks for `Put` operations. If a version is not provided, it performs a Read-Modify-Write to automatically fetch the current key version before writing, helping ensure linearizability.
3. **Leader Tracking:** Interacts with the active Raft leader of the replica groups, retrying alternative group members if contacting a follower or if a network partition occurs.

---

## Usage

```bash
# Get the value of a key
dkv-client --ctrler-addr <controller-ip:port> get <key>

# Put a key-value pair (auto-fetches version for Read-Modify-Write)
dkv-client --ctrler-addr <controller-ip:port> put <key> <value>

# Put a key-value pair with an explicit version guard
dkv-client --ctrler-addr <controller-ip:port> put <key> <value> <version>
```

---

## Command Line Flags

* `--ctrler-addr`: Specifies the address of the Shard Controller (default is `localhost:9000` or read from the `DKV_CTRLED_ADDR` environment variable).
