package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	
	"github.com/sjohri/dcache/internal/cache"
	"github.com/sjohri/dcache/internal/cluster"
	"github.com/sjohri/dcache/internal/server"
)

// NodeStatus string representation
func nodeStatusString(status cluster.NodeStatus) string {
	switch status {
	case cluster.NodeHealthy:
		return "healthy"
	case cluster.NodeSuspect:
		return "suspect"
	case cluster.NodeDead:
		return "dead"
	default:
		return "unknown"
	}
}
// Config holds server configuration
type Config struct {
	HTTPPort     string
	TCPPort      string
	Capacity     int
	MaxSizeMB    int64
	DefaultTTL   time.Duration
	NodeID       string
	
	// Cluster configuration
	ClusterMode  bool
	GossipPort   string
	SeedNodes    []string
	VirtualNodes int
}

var startTime = time.Now()

func main() {
	// Parse command line flags
	config := parseFlags()
	
	// Print startup banner
	printBanner(config)
	
	// Create cache store
	cacheStore := cache.NewStore(
		config.Capacity,
		config.MaxSizeMB,
		config.DefaultTTL,
	)
	
	// Initialize cluster components if in cluster mode
	var nodeManager *cluster.NodeManager
	var gossip *cluster.GossipProtocol
	
	if config.ClusterMode {
		// Initialize node manager
		nodeManager = cluster.NewNodeManager(config.NodeID, config.VirtualNodes)
		
		// Start gossip protocol
		gossipAddr := fmt.Sprintf(":%s", config.GossipPort)
		gossip = cluster.NewGossipProtocol(nodeManager, gossipAddr, config.SeedNodes)
		
		if err := gossip.Start(); err != nil {
			log.Fatalf("Failed to start gossip protocol: %v", err)
		}
		
		log.Printf("Cluster mode enabled with %d virtual nodes", config.VirtualNodes)
		log.Printf("Gossip listening on port %s", config.GossipPort)
		if len(config.SeedNodes) > 0 {
			log.Printf("Seed nodes: %v", config.SeedNodes)
		}
	}
	
	// Start TCP server
	tcpServer := server.NewServer(":"+config.TCPPort, cacheStore)
	if err := tcpServer.Start(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}
	
	// Start HTTP server (for easy testing and monitoring)
	httpServer := startHTTPServer(config, cacheStore, nodeManager)
	
	// Wait for shutdown signal
	waitForShutdown(tcpServer, httpServer, gossip)
}

func parseFlags() *Config {
	config := &Config{}
	
	// Basic configuration
	flag.StringVar(&config.HTTPPort, "http-port", getEnv("HTTP_PORT", "8080"), "HTTP server port")
	flag.StringVar(&config.TCPPort, "tcp-port", getEnv("TCP_PORT", "6379"), "TCP server port")
	flag.IntVar(&config.Capacity, "capacity", getEnvInt("CAPACITY", 10000), "Max number of items")
	flag.Int64Var(&config.MaxSizeMB, "max-size", getEnvInt64("MAX_SIZE_MB", 100), "Max cache size in MB")
	
	ttlMinutes := flag.Int("ttl", getEnvInt("DEFAULT_TTL_MIN", 10), "Default TTL in minutes")
	flag.StringVar(&config.NodeID, "node-id", getEnv("NODE_ID", "node-1"), "Node identifier")
	
	// Cluster configuration
	flag.BoolVar(&config.ClusterMode, "cluster", getEnvBool("CLUSTER_MODE", false), "Enable cluster mode")
	flag.StringVar(&config.GossipPort, "gossip-port", getEnv("GOSSIP_PORT", "7946"), "Gossip protocol port")
	flag.IntVar(&config.VirtualNodes, "virtual-nodes", getEnvInt("VIRTUAL_NODES", 150), "Virtual nodes per physical node")
	
	flag.Parse()
	
	config.DefaultTTL = time.Duration(*ttlMinutes) * time.Minute
	
	// Parse seed nodes from environment or flags
	if seedNodesEnv := os.Getenv("SEED_NODES"); seedNodesEnv != "" {
		config.SeedNodes = strings.Split(seedNodesEnv, ",")
		// Trim spaces from each seed node
		for i := range config.SeedNodes {
			config.SeedNodes[i] = strings.TrimSpace(config.SeedNodes[i])
		}
	}
	
	return config
}

func printBanner(config *Config) {
	fmt.Println("DCache Server v1.0")
	fmt.Println("==================")
	fmt.Printf("Node ID:      %s\n", config.NodeID)
	fmt.Printf("HTTP Port:    %s\n", config.HTTPPort)
	fmt.Printf("TCP Port:     %s\n", config.TCPPort)
	fmt.Printf("Capacity:     %d items\n", config.Capacity)
	fmt.Printf("Max Size:     %d MB\n", config.MaxSizeMB)
	fmt.Printf("Default TTL:  %v\n", config.DefaultTTL)
	
	if config.ClusterMode {
		fmt.Printf("Cluster:      Enabled\n")
		fmt.Printf("Gossip Port:  %s\n", config.GossipPort)
		fmt.Printf("Virtual Nodes: %d\n", config.VirtualNodes)
	}
	
	fmt.Println()
}

func startHTTPServer(config *Config, cacheStore *cache.Store, nodeManager *cluster.NodeManager) *http.Server {
	mux := http.NewServeMux()
	
	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":    "healthy",
			"node_id":   config.NodeID,
			"timestamp": time.Now().Unix(),
		}
		
		if config.ClusterMode {
			response["cluster_mode"] = true
			response["cluster_size"] = len(nodeManager.GetAllNodes())
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	
	// Stats endpoint
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := cacheStore.Stats()
		stats["node_id"] = config.NodeID
		stats["uptime"] = time.Since(startTime).Seconds()
		
		if config.ClusterMode {
			stats["cluster_mode"] = true
			stats["cluster_nodes"] = len(nodeManager.GetAllNodes())
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})
	
	// Cache operations via HTTP (for testing)
	mux.HandleFunc("/cache/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/cache/"):]
		
		// In cluster mode, check if this is the right node
		if config.ClusterMode && nodeManager != nil {
			targetNode, err := nodeManager.GetNodeForKey(key)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(fmt.Sprintf("Cluster error: %v\n", err)))
				return
			}
			
			// If not our key, redirect to correct node
			if !nodeManager.IsLocalNode(targetNode.ID) {
				w.Header().Set("X-Redirect-Node", targetNode.ID)
				w.Header().Set("X-Redirect-Address", targetNode.Address)
				w.WriteHeader(http.StatusTemporaryRedirect)
				w.Write([]byte(fmt.Sprintf("Key belongs to node %s\n", targetNode.ID)))
				return
			}
		}
		
		switch r.Method {
		case http.MethodGet:
			handleHTTPGet(w, r, cacheStore, key)
		case http.MethodPost, http.MethodPut:
			handleHTTPSet(w, r, cacheStore, key)
		case http.MethodDelete:
			handleHTTPDelete(w, r, cacheStore, key)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			w.Write([]byte("Method not allowed"))
		}
	})
	
	// Keys endpoint (debugging)
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		keys := cacheStore.Keys()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"keys":  keys,
			"count": len(keys),
		})
	})
	
	// Clear cache endpoint
	mux.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		cacheStore.Clear()
		w.Write([]byte("Cache cleared\n"))
	})
	
	// Cluster endpoints (only if cluster mode is enabled)
	if config.ClusterMode && nodeManager != nil {
		// Get cluster nodes
		mux.HandleFunc("/cluster/nodes", func(w http.ResponseWriter, r *http.Request) {
			nodes := nodeManager.GetAllNodes()
			
			var response []map[string]interface{}
			for _, node := range nodes {
				nodeInfo := map[string]interface{}{
					"id":        node.ID,
					"address":   node.Address,
					"status":    nodeStatusString(node.Status),  // Use the function here
					"last_seen": node.LastSeen.Format(time.RFC3339),
				}
				
				// Mark local node
				if nodeManager.IsLocalNode(node.ID) {
					nodeInfo["local"] = true
				}
				
				response = append(response, nodeInfo)
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		})
		
		// Get hash ring distribution
		mux.HandleFunc("/cluster/ring", func(w http.ResponseWriter, r *http.Request) {
			hashRing := nodeManager.GetHashRing()
			if hashRing == nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("Hash ring not available\n"))
				return
			}
			
			stats := hashRing.Stats()
			
			// Calculate percentages
			total := 0
			for _, count := range stats {
				total += count
			}
			
			distribution := make(map[string]interface{})
			for node, count := range stats {
				distribution[node] = map[string]interface{}{
					"virtual_nodes": count,
					"percentage":   float64(count) * 100 / float64(total),
				}
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(distribution)
		})
		
		// Test key routing
		mux.HandleFunc("/cluster/locate/", func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Path[len("/cluster/locate/"):]
			
			node, err := nodeManager.GetNodeForKey(key)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(fmt.Sprintf("Error: %v\n", err)))
				return
			}
			
			response := map[string]interface{}{
				"key":     key,
				"node_id": node.ID,
				"address": node.Address,
				"local":   nodeManager.IsLocalNode(node.ID),
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		})
	}
	
	server := &http.Server{
		Addr:    ":" + config.HTTPPort,
		Handler: mux,
	}
	
	go func() {
		log.Printf("HTTP server listening on :%s", config.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	
	return server
}

func handleHTTPGet(w http.ResponseWriter, _ *http.Request, cache *cache.Store, key string) {
	value, found := cache.Get(key)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Key not found\n"))
		return
	}
	
	w.Header().Set("X-Cache-Hit", "true")
	w.Write(value)
}

func handleHTTPSet(w http.ResponseWriter, r *http.Request, cache *cache.Store, key string) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Failed to read body\n"))
		return
	}
	defer r.Body.Close()
	
	// Check for TTL header
	var ttl time.Duration
	if ttlStr := r.Header.Get("X-TTL-Seconds"); ttlStr != "" {
		if seconds, err := strconv.Atoi(ttlStr); err == nil {
			ttl = time.Duration(seconds) * time.Second
		}
	}
	
	// Store in cache
	if ttl > 0 {
		err = cache.SetWithTTL(key, body, ttl)
	} else {
		err = cache.Set(key, body)
	}
	
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Failed to store: %v\n", err)))
		return
	}
	
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Stored\n"))
}

func handleHTTPDelete(w http.ResponseWriter, _ *http.Request, cache *cache.Store, key string) {
	if cache.Delete(key) {
		w.Write([]byte("Deleted\n"))
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Key not found\n"))
	}
}

func waitForShutdown(tcpServer *server.Server, httpServer *http.Server, gossip *cluster.GossipProtocol) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	
	// Graceful shutdown
	log.Println("Shutting down servers...")
	
	// Leave cluster if in cluster mode
	if gossip != nil {
		log.Println("Leaving cluster...")
		gossip.Leave()
		time.Sleep(500 * time.Millisecond) // Give time for leave message to propagate
		gossip.Stop()
	}
	
	// Stop TCP server
	if err := tcpServer.Stop(); err != nil {
		log.Printf("TCP server shutdown error: %v", err)
	}
	
	// Stop HTTP server
	if err := httpServer.Close(); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	
	log.Println("Shutdown complete")
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
