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
	"syscall"
	"time"
	
	"github.com/sjohri/dcache/internal/cache"
	"github.com/sjohri/dcache/internal/server"
)

// Config holds server configuration
type Config struct {
	HTTPPort   string
	TCPPort    string
	Capacity   int
	MaxSizeMB  int64
	DefaultTTL time.Duration
	NodeID     string
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
	
	// Start TCP server
	tcpServer := server.NewServer(":"+config.TCPPort, cacheStore)
	if err := tcpServer.Start(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}
	
	// Start HTTP server (for easy testing and monitoring)
	httpServer := startHTTPServer(config, cacheStore)
	
	// Wait for shutdown signal
	waitForShutdown(tcpServer, httpServer)
}

func parseFlags() *Config {
	config := &Config{}
	
	flag.StringVar(&config.HTTPPort, "http-port", getEnv("HTTP_PORT", "8080"), "HTTP server port")
	flag.StringVar(&config.TCPPort, "tcp-port", getEnv("TCP_PORT", "6379"), "TCP server port")
	flag.IntVar(&config.Capacity, "capacity", getEnvInt("CAPACITY", 10000), "Max number of items")
	flag.Int64Var(&config.MaxSizeMB, "max-size", getEnvInt64("MAX_SIZE_MB", 100), "Max cache size in MB")
	
	ttlMinutes := flag.Int("ttl", getEnvInt("DEFAULT_TTL_MIN", 10), "Default TTL in minutes")
	flag.StringVar(&config.NodeID, "node-id", getEnv("NODE_ID", "node-1"), "Node identifier")
	
	flag.Parse()
	
	config.DefaultTTL = time.Duration(*ttlMinutes) * time.Minute
	
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
	fmt.Println()
}

func startHTTPServer(config *Config, cacheStore *cache.Store) *http.Server {
	mux := http.NewServeMux()
	
	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status":    "healthy",
			"node_id":   config.NodeID,
			"timestamp": time.Now().Unix(),
		}
		json.NewEncoder(w).Encode(response)
	})
	
	// Stats endpoint
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := cacheStore.Stats()
		stats["node_id"] = config.NodeID
		stats["uptime"] = time.Since(startTime).Seconds()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})
	
	// Cache operations via HTTP (for testing)
	mux.HandleFunc("/cache/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/cache/"):]
		
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

func handleHTTPGet(w http.ResponseWriter, r *http.Request, cache *cache.Store, key string) {
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

func handleHTTPDelete(w http.ResponseWriter, r *http.Request, cache *cache.Store, key string) {
	if cache.Delete(key) {
		w.Write([]byte("Deleted\n"))
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Key not found\n"))
	}
}

func waitForShutdown(tcpServer *server.Server, httpServer *http.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	sig := <-sigChan
	log.Printf("Received signal: %v", sig)
	
	// Graceful shutdown
	log.Println("Shutting down servers...")
	
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
