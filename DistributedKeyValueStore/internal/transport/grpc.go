package transport

import (
	"bufio"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	envMap     map[string]string
	envMapOnce sync.Once
)

func loadEnvFile() {
	envMap = make(map[string]string)
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			file, err := os.Open(envPath)
			if err != nil {
				break
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if strings.Contains(line, "=") {
					parts := strings.SplitN(line, "=", 2)
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					envMap[key] = val
				}
			}
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}

// GetEnv returns the environment variable or .env value, falling back to defaultVal.
func GetEnv(key string, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	envMapOnce.Do(loadEnvFile)
	if val, ok := envMap[key]; ok {
		return val
	}
	return defaultVal
}

func translateAddr(addr string) string {
	return GetEnv("MAP_"+addr, addr)
}

// GRPCNetwork implements core.Network using Go net/rpc over TCP connection pool.
type GRPCNetwork struct {
	mu    sync.Mutex
	conns map[string]*rpc.Client
}

// NewGRPCNetwork creates a network transport based on TCP connection pooling.
func NewGRPCNetwork() *GRPCNetwork {
	return &GRPCNetwork{
		conns: make(map[string]*rpc.Client),
	}
}

// Call sends an RPC to the peer.
func (gn *GRPCNetwork) Call(peerAddr string, method string, args interface{}, reply interface{}) bool {
	var client *rpc.Client
	var err error

	gn.mu.Lock()
	client = gn.conns[peerAddr]
	gn.mu.Unlock()

	if client == nil {
		translatedAddr := translateAddr(peerAddr)
		client, err = rpc.Dial("tcp", translatedAddr)
		if err != nil {
			return false
		}
		gn.mu.Lock()
		gn.conns[peerAddr] = client
		gn.mu.Unlock()
	}

	// Make RPC call
	err = client.Call(method, args, reply)
	if err != nil {
		// Connection might be broken, close and remove from pool
		client.Close()
		gn.mu.Lock()
		delete(gn.conns, peerAddr)
		gn.mu.Unlock()

		// Retry once
		translatedAddr := translateAddr(peerAddr)
		client, err = rpc.Dial("tcp", translatedAddr)
		if err != nil {
			return false
		}
		gn.mu.Lock()
		gn.conns[peerAddr] = client
		gn.mu.Unlock()

		err = client.Call(method, args, reply)
		return err == nil
	}

	return true
}

// Close shuts down all connections in the pool.
func (gn *GRPCNetwork) Close() {
	gn.mu.Lock()
	for _, client := range gn.conns {
		client.Close()
	}
	gn.conns = make(map[string]*rpc.Client)
	gn.mu.Unlock()
}

// StartServer starts a TCP server for a given node address and registers the services.
func StartServer(addr string, services map[string]interface{}) (net.Listener, error) {
	server := rpc.NewServer()
	for name, svc := range services {
		if err := server.RegisterName(name, svc); err != nil {
			return nil, err
		}
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go server.ServeConn(conn)
		}
	}()

	return l, nil
}
