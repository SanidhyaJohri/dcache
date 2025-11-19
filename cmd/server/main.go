package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Version information
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "dev"
	}

	fmt.Printf("Starting dcache server...\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Node ID: %s\n", nodeID)
	fmt.Printf("Port: %s\n", port)

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := HealthResponse{
			Status:    "healthy",
			Version:   Version,
			NodeID:    nodeID,
			Timestamp: time.Now(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Basic cache endpoints (placeholder)
	http.HandleFunc("/cache/", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path[len("/cache/"):]

		switch r.Method {
		case http.MethodGet:
			w.Write([]byte(fmt.Sprintf("GET key: %s from node: %s\n", key, nodeID)))
		case http.MethodPut, http.MethodPost:
			w.Write([]byte(fmt.Sprintf("SET key: %s on node: %s\n", key, nodeID)))
		case http.MethodDelete:
			w.Write([]byte(fmt.Sprintf("DELETE key: %s from node: %s\n", key, nodeID)))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
