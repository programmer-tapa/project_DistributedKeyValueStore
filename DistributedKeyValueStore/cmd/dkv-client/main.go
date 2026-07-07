// Command dkv-client provides a CLI client for interacting with the
// distributed key/value store.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"dkv/internal/core"
	"dkv/internal/shardkv"
	"dkv/internal/transport"
)

func main() {
	// 1. Parse configuration flags. 
	// Default to using the DKV_CTRLED_ADDR env variable (e.g. for Docker compose), falling back to localhost:9000.
	defaultCtrlerAddr := transport.GetEnv("DKV_CTRLED_ADDR", "localhost:9000")
	ctrlerAddr := flag.String("ctrler-addr", defaultCtrlerAddr, "Shard controller address")
	flag.Parse()

	// Ensure we have at least: <command> <key>
	args := flag.Args()
	if len(args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// 2. Initialize the gRPC network transport layer.
	netTrans := transport.NewGRPCNetwork()
	defer netTrans.Close()

	// 3. Create the ShardKV Clerk.
	// This Clerk queries the ShardController to discover the topology and
	// routes key-value requests to the appropriate replica group.
	ck := shardkv.NewClerk(*ctrlerAddr, netTrans)

	cmd := args[0]
	switch cmd {
	case "get":
		// Handle GET request: dkv-client get <key>
		key := args[1]
		val, ver, err := ck.Get(key)
		if err == core.ErrNoKey {
			fmt.Printf("Key not found: %s\n", key)
			os.Exit(1)
		} else if err != core.OK {
			log.Fatalf("Error getting key: %v", err)
		}
		// Output the fetched value along with its current version.
		fmt.Printf("Value: %s (Version: %d)\n", val, ver)

	case "put":
		// Handle PUT request: dkv-client put <key> <value> [version]
		if len(args) < 3 {
			fmt.Println("Usage: put <key> <value> [version]")
			os.Exit(1)
		}
		key := args[1]
		value := args[2]

		var version uint64
		if len(args) == 4 {
			// If version is provided explicitly by the user, parse and use it.
			v, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				log.Fatalf("Invalid version: %s", args[3])
			}
			version = v
		} else {
			// If no version is provided, perform a Read-Modify-Write:
			// Fetch the key's current version first to ensure write linearizability.
			_, ver, err := ck.Get(key)
			if err == core.ErrNoKey {
				// Key does not exist yet; version 0 specifies a "create" operation.
				version = 0
			} else if err == core.OK {
				// Use the currently stored version to assert we are overwriting
				// the latest known state.
				version = ver
			} else {
				log.Fatalf("failed to auto-fetch version: %v", err)
			}
		}

		// Perform the version-guarded conditional Put operation.
		err := ck.Put(key, value, version)
		if err != core.OK {
			log.Fatalf("Put failed: %v", err)
		}
		fmt.Println("OK")

	default:
		printUsage()
		os.Exit(1)
	}
}

// printUsage displays command line instructions to stdout.
func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  dkv-client --ctrler-addr <addr> get <key>")
	fmt.Println("  dkv-client --ctrler-addr <addr> put <key> <value> [version]")
}
