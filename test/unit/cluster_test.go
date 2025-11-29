package test

import (
	"fmt"
	"testing"
	
	"github.com/sjohri/dcache/internal/cluster"
	"github.com/sjohri/dcache/pkg/client"
)

func TestConsistentHashing(t *testing.T) {
	ring := cluster.NewHashRing(150)
	
	// Add nodes
	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")
	
	// Test key distribution
	distribution := make(map[string]int)
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key-%d", i)
		node := ring.GetNode(key)
		distribution[node]++
	}
	
	// Check even distribution (within 20% variance)
	avg := 10000 / 3
	for node, count := range distribution {
		variance := float64(abs(count-avg)) / float64(avg)
		if variance > 0.2 {
			t.Errorf("Node %s has poor distribution: %d keys (%.2f%% variance)",
				node, count, variance*100)
		}
	}
}

func TestNodeFailure(t *testing.T) {
	// Create node manager
	nm := cluster.NewNodeManager("test-node", 150)
	
	// Add nodes
	nm.AddNode(&cluster.Node{
		ID:      "node1",
		Address: "localhost:8001",
		Status:  cluster.NodeHealthy,
	})
	
	nm.AddNode(&cluster.Node{
		ID:      "node2",
		Address: "localhost:8002",
		Status:  cluster.NodeHealthy,
	})
	
	// Get node for a key
	node1, _ := nm.GetNodeForKey("test-key")
	
	// Remove a node
	nm.RemoveNode("node1")
	
	// Key should map to different node
	node2, _ := nm.GetNodeForKey("test-key")
	
	if node1 == node2 {
		t.Error("Key should remap after node removal")
	}
}

func TestSmartClient(t *testing.T) {
	// This would require running actual nodes
	// For unit testing, we'd mock the HTTP responses
	
	client := client.NewSmartClient([]string{
		"localhost:8001",
		"localhost:8002",
		"localhost:8003",
	})
	
	// Test key routing
	distribution := client.GetNodeDistribution(1000)
	
	// Verify distribution
	for node, count := range distribution {
		t.Logf("Node %s: %d keys", node, count)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
