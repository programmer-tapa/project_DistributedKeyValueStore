// Command kvraftd starts a fault-tolerant KV server backed by Raft.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"dkv/internal/core"
	"dkv/internal/persist"
	"dkv/internal/raft"
	"dkv/internal/rsm"
	"dkv/internal/shardgrp"
	"dkv/internal/transport"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 1. Parse command-line configuration flags.
	me := flag.Int("me", -1, "Node index in peers list")
	peersStr := flag.String("peers", "", "Comma-separated list of peer addresses")
	gid := flag.Int("gid", 0, "Group GID")
	persistDir := flag.String("persist-dir", "", "State persistence directory")
	maxState := flag.Int("max-raft-state", 10000, "Max Raft state bytes before snapshot")
	metricsAddr := flag.String("metrics-addr", "", "Prometheus metrics address")
	flag.Parse()

	// Enforce required flags
	if *me == -1 || *peersStr == "" || *persistDir == "" {
		log.Fatalf("missing required flags: --me, --peers, and --persist-dir are required")
	}

	peers := strings.Split(*peersStr, ",")
	if *me < 0 || *me >= len(peers) {
		log.Fatalf("invalid index --me: %d (must be between 0 and %d)", *me, len(peers)-1)
	}

	// 2. Initialize the gRPC/TCP network transport layer.
	netTrans := transport.NewGRPCNetwork()
	defer netTrans.Close()

	// 3. Create the persistent storage layer.
	// Used by Raft to persist consensus state (log, currentTerm, votedFor) and database snapshots.
	pers := persist.NewDiskPersister(*persistDir)
	applyCh := make(chan core.ApplyMsg, 1000)

	// 4. Instantiate the Raft consensus engine node.
	rf := raft.Make(peers, *me, pers, netTrans, applyCh)

	// 5. Instantiate the ShardGroup replicated state machine.
	sg := shardgrp.New(*gid)

	// 6. Instantiate the Replicated State Machine (RSM) bridge.
	// It consumes committed log entries from `applyCh` and applies them to `shardgrp.ShardGroup`.
	rsmObj := rsm.New(rf, sg, *maxState, applyCh)
	sg.SetRSM(rsmObj)

	// 7. Start HTTP server to expose Prometheus metrics (if configured).
	if *metricsAddr != "" {
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			log.Printf("Prometheus metrics listening on %s", *metricsAddr)
			if err := http.ListenAndServe(*metricsAddr, nil); err != nil {
				log.Printf("metrics server error: %v", err)
			}
		}()
	}

	// 8. Map TCP RPC services to their respective handlers.
	// Clients/Peers call these endpoints.
	services := map[string]interface{}{
		"Raft":       rf,
		"ShardGroup": sg,
	}

	// Start the TCP server listener.
	listener, err := transport.StartServer(peers[*me], services)
	if err != nil {
		log.Fatalf("failed to start RPC server on %s: %v", peers[*me], err)
	}
	defer listener.Close()

	log.Printf("kvraftd node %d listening on %s (GID: %d)", *me, peers[*me], *gid)

	// 9. Block main execution until signal is received, triggering clean termination of Raft.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	rf.Kill()
}
