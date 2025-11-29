package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
	
	"github.com/sjohri/dcache/internal/cluster"
)

// SmartClient is a cluster-aware cache client
type SmartClient struct {
	hashRing    *cluster.HashRing
	nodes       map[string]string // nodeID -> address
	httpClient  *http.Client
	mu          sync.RWMutex
}

// NewSmartClient creates a new smart client
func NewSmartClient(seedNodes []string) *SmartClient {
	client := &SmartClient{
		hashRing:   cluster.NewHashRing(150),
		nodes:      make(map[string]string),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
	
	// Initialize with seed nodes
	for i, addr := range seedNodes {
		nodeID := fmt.Sprintf("node%d", i+1)
		client.AddNode(nodeID, addr)
	}
	
	// Start topology updater
	go client.updateTopology()
	
	return client
}

// AddNode adds a node to the client's view
func (c *SmartClient) AddNode(nodeID, address string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.nodes[nodeID] = address
	c.hashRing.AddNode(nodeID)
}

// RemoveNode removes a node from the client's view
func (c *SmartClient) RemoveNode(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.nodes, nodeID)
	c.hashRing.RemoveNode(nodeID)
}

// Get retrieves a value from the appropriate node
func (c *SmartClient) Get(key string) ([]byte, error) {
	node := c.getNodeForKey(key)
	if node == "" {
		return nil, fmt.Errorf("no nodes available")
	}
	
	c.mu.RLock()
	address := c.nodes[node]
	c.mu.RUnlock()
	
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/cache/%s", address, key))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("key not found")
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error: %d", resp.StatusCode)
	}
	
	return io.ReadAll(resp.Body)
}

// Set stores a value on the appropriate node
func (c *SmartClient) Set(key string, value []byte) error {
	return c.SetWithTTL(key, value, 0)
}

// SetWithTTL stores a value with TTL on the appropriate node
func (c *SmartClient) SetWithTTL(key string, value []byte, ttl time.Duration) error {
	node := c.getNodeForKey(key)
	if node == "" {
		return fmt.Errorf("no nodes available")
	}
	
	c.mu.RLock()
	address := c.nodes[node]
	c.mu.RUnlock()
	
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://%s/cache/%s", address, key),
		bytes.NewReader(value),
	)
	if err != nil {
		return err
	}
	
	if ttl > 0 {
		req.Header.Set("X-TTL-Seconds", fmt.Sprintf("%d", int(ttl.Seconds())))
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set key: status %d", resp.StatusCode)
	}
	
	return nil
}

// Delete removes a key from the appropriate node
func (c *SmartClient) Delete(key string) error {
	node := c.getNodeForKey(key)
	if node == "" {
		return fmt.Errorf("no nodes available")
	}
	
	c.mu.RLock()
	address := c.nodes[node]
	c.mu.RUnlock()
	
	req, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("http://%s/cache/%s", address, key),
		nil,
	)
	if err != nil {
		return err
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("failed to delete key: status %d", resp.StatusCode)
	}
	
	return nil
}

// GetStats retrieves statistics from all nodes
func (c *SmartClient) GetStats() (map[string]interface{}, error) {
	c.mu.RLock()
	nodes := make(map[string]string)
	for k, v := range c.nodes {
		nodes[k] = v
	}
	c.mu.RUnlock()
	
	allStats := make(map[string]interface{})
	
	for nodeID, address := range nodes {
		resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/stats", address))
		if err != nil {
			continue // Skip failed nodes
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			var stats map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&stats); err == nil {
				allStats[nodeID] = stats
			}
		}
	}
	
	return allStats, nil
}

// getNodeForKey returns the node responsible for a key
func (c *SmartClient) getNodeForKey(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.hashRing.GetNode(key)
}

// updateTopology periodically updates the cluster topology
func (c *SmartClient) updateTopology() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		// Get cluster info from a random node
		c.mu.RLock()
		var sampleNode string
		var sampleAddr string
		for nodeID, addr := range c.nodes {
			sampleNode = nodeID
			sampleAddr = addr
			break
		}
		c.mu.RUnlock()
		
		if sampleNode == "" {
			continue
		}
		
		// Get cluster topology from the sample node
		resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/cluster/nodes", sampleAddr))
		if err != nil {
			continue
		}
		
		var nodes []struct {
			ID      string `json:"id"`
			Address string `json:"address"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		
		// Update local topology
		c.mu.Lock()
		
		// Remove nodes not in the new list
		newNodes := make(map[string]bool)
		for _, node := range nodes {
			newNodes[node.ID] = true
		}
		
		for nodeID := range c.nodes {
			if !newNodes[nodeID] {
				delete(c.nodes, nodeID)
				c.hashRing.RemoveNode(nodeID)
			}
		}
		
		// Add new nodes
		for _, node := range nodes {
			if _, exists := c.nodes[node.ID]; !exists {
				c.nodes[node.ID] = node.Address
				c.hashRing.AddNode(node.ID)
			}
		}
		
		c.mu.Unlock()
	}
}

// GetNodeDistribution returns key distribution across nodes
func (c *SmartClient) GetNodeDistribution(sampleKeys int) map[string]int {
	distribution := make(map[string]int)
	
	for i := 0; i < sampleKeys; i++ {
		key := fmt.Sprintf("sample-key-%d", i)
		node := c.getNodeForKey(key)
		distribution[node]++
	}
	
	return distribution
}
