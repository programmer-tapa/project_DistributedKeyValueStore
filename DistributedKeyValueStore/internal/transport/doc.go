// Package transport provides concrete implementations of core.Network.
//
// The transport abstraction allows the same Raft/KV code to run:
//   - In tests: via in-memory channels (simulating network partitions, delays)
//   - In production: via gRPC over TCP (real network, Docker containers)
//
// This is the key production enhancement over the academic lab, which
// only supports the in-memory labrpc simulator.
package transport
