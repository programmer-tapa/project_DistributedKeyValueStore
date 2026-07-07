// Command kvsrvd starts a single-node linearizable key/value server.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"dkv/internal/kvsrv"
	"dkv/internal/transport"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 1. Parse command-line configuration flags.
	addr := flag.String("addr", ":9000", "Listen address")
	metricsAddr := flag.String("metrics-addr", "", "Prometheus metrics address")
	flag.Parse()

	// 2. Instantiate the single-node key/value server.
	// In the sharded architecture, this node acts as the central Config Metadata Server.
	kv := kvsrv.New()
	
	// Map the RPC service name "KVServer" to our instanced handler
	services := map[string]interface{}{
		"KVServer": kv,
	}

	// 3. Start Prometheus metrics reporting asynchronously if the flag is provided.
	if *metricsAddr != "" {
		go func() {
			http.Handle("/metrics", promhttp.Handler())
			log.Printf("Prometheus metrics listening on %s", *metricsAddr)
			if err := http.ListenAndServe(*metricsAddr, nil); err != nil {
				log.Printf("metrics server error: %v", err)
			}
		}()
	}

	// 4. Start the TCP RPC listener to expose the KVServer methods.
	listener, err := transport.StartServer(*addr, services)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("kvsrvd listening on %s", *addr)

	// 5. Block main goroutine until SIGINT or SIGTERM is caught, prompting a clean exit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
