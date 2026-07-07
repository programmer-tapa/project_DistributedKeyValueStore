// Command shardctrlrd starts the shard controller process.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dkv/internal/core"
	"dkv/internal/metrics"
	"dkv/internal/shardctrler"
	"dkv/internal/transport"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// parseGroups deserializes the GID-to-address-list string parameter into a structured map.
// The input format is "GID1=addr1,addr2;GID2=addr3".
func parseGroups(s string) (map[int][]string, error) {
	groups := make(map[int][]string)
	if s == "" {
		return groups, nil
	}
	parts := strings.Split(s, ";")
	for _, part := range parts {
		if part == "" {
			continue
		}
		sub := strings.Split(part, "=")
		if len(sub) != 2 {
			return nil, fmt.Errorf("invalid group format: %s", part)
		}
		gid, err := strconv.Atoi(sub[0])
		if err != nil {
			return nil, fmt.Errorf("invalid GID: %s", sub[0])
		}
		servers := strings.Split(sub[1], ",")
		groups[gid] = servers
	}
	return groups, nil
}

func main() {
	// 1. Parse command line flags.
	kvsrvAddr := flag.String("kvsrv-addr", "localhost:9000", "Address of config store (kvsrv)")
	groupsStr := flag.String("groups", "", "GIDs to server addresses e.g. 101=g1-0:9001,g1-1:9001;102=g2-0:9002")
	metricsAddr := flag.String("metrics-addr", ":2112", "Address to expose Prometheus metrics")
	flag.Parse()

	// Parse the mapped group configurations
	groups, err := parseGroups(*groupsStr)
	if err != nil {
		log.Fatalf("failed to parse groups: %v", err)
	}

	// 2. Initialize the gRPC network transport layer.
	netTrans := transport.NewGRPCNetwork()
	defer netTrans.Close()

	// 3. Create the ShardController orchestrator instance.
	sc := shardctrler.New(*kvsrvAddr, groups, netTrans)

	// 4. Start HTTP server to expose Prometheus metrics.
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Printf("Prometheus metrics listening on %s", *metricsAddr)
		if err := http.ListenAndServe(*metricsAddr, nil); err != nil {
			log.Printf("metrics server error: %v", err)
		}
	}()

	// 5. Query and evaluate the current cluster configuration on startup.
	curr, err := sc.Query()
	if err != nil {
		log.Fatalf("failed to query config: %v", err)
	}

	if curr.Num == -1 {
		// No config exists yet; initialize the metadata store with the parsed group addresses.
		log.Println("Initializing configuration...")
		if err := sc.InitConfig(groups); err != nil {
			log.Fatalf("failed to init config: %v", err)
		}
		log.Println("Configuration initialized successfully.")
	} else {
		// A configuration already exists. Compare it with the input flags to detect membership changes.
		differ := false
		if len(curr.Groups) != len(groups) {
			differ = true
		} else {
			// Deep compare GID memberships and server address orders
			for gid, servers := range groups {
				currServers, ok := curr.Groups[gid]
				if !ok || len(currServers) != len(servers) {
					differ = true
					break
				}
				for i := range servers {
					if servers[i] != currServers[i] {
						differ = true
						break
					}
				}
			}
		}

		if differ {
			// Configuration mismatch detected. Begin executing a cluster-wide shard rebalancing migration.
			log.Println("Detected configuration change. Migrating shards...")
			target := core.ShardConfig{
				Num:    curr.Num + 1,
				Groups: groups,
			}
			var gids []int
			for gid := range groups {
				gids = append(gids, gid)
			}
			sort.Ints(gids)

			// Simple, deterministic round-robin shard assignment based on active GIDs
			if len(gids) > 0 {
				for i := 0; i < core.NShards; i++ {
					target.Shards[i] = gids[i%len(gids)]
				}
			}

			start := time.Now()
			metrics.ShardMigrations.WithLabelValues("freeze").Inc()
			
			// Invoke the four-phase migration protocol to move keys safely between replica groups
			if err := sc.ChangeConfigTo(target); err != nil {
				metrics.RPCErrors.WithLabelValues("ChangeConfigTo", "migration_error").Inc()
				log.Fatalf("failed to change config: %v", err)
			}
			
			metrics.ShardMigrationDuration.Observe(time.Since(start).Seconds())
			metrics.ShardMigrations.WithLabelValues("complete").Inc()
			log.Println("Migration completed successfully.")
		} else {
			log.Println("Configuration is up to date.")
		}
	}

	// 6. Start background goroutine to periodically report the current configuration version.
	go func() {
		for {
			if cfg, err := sc.Query(); err == nil {
				metrics.ShardConfigNum.Set(float64(cfg.Num))
			}
			time.Sleep(5 * time.Second)
		}
	}()

	// 7. Wait for system signals (SIGINT, SIGTERM) to cleanly shut down.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
