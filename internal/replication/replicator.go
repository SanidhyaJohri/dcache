package replication

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/sjohri/dcache/internal/cluster"
	"github.com/sjohri/dcache/internal/consistency"
)

// ReplicationConfig holds replication settings
type ReplicationConfig struct {
	ReplicationFactor int
	WriteQuorum       int
	ReadQuorum        int
	Timeout           time.Duration
}

// Replicator manages data replication across nodes
type Replicator struct {
	config      *ReplicationConfig
	nodeManager *cluster.NodeManager
	httpClient  *http.Client
	mu          sync.RWMutex
}

// NewReplicator creates a new replicator
func NewReplicator(config *ReplicationConfig, nodeManager *cluster.NodeManager) *Replicator {
	return &Replicator{
		config:      config,
		nodeManager: nodeManager,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ReplicatedValue represents a value with metadata
type ReplicatedValue struct {
	Data        []byte                   `json:"data"`
	VectorClock *consistency.VectorClock `json:"vector_clock"`
	Timestamp   time.Time                `json:"timestamp"`
}

// Write performs a quorum write to replicas
func (r *Replicator) Write(key string, value []byte) (*consistency.VectorClock, error) {
	// Get replica nodes
	replicas, err := r.getReplicaNodes(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get replicas: %v", err)
	}

	// Create new vector clock
	localNodeID := r.nodeManager.GetLocalNodeID()
	vc := consistency.NewVectorClock()
	vc.Increment(localNodeID)

	// Prepare replicated value
	repValue := &ReplicatedValue{
		Data:        value,
		VectorClock: vc,
		Timestamp:   time.Now(),
	}

	// Write to replicas
	var wg sync.WaitGroup
	successChan := make(chan bool, len(replicas))

	for _, replica := range replicas {
		wg.Add(1)
		go func(node *cluster.Node) {
			defer wg.Done()

			if r.nodeManager.IsLocalNode(node.ID) {
				// Local write (handled by caller)
				successChan <- true
				return
			}

			// Remote write
			if err := r.writeToNode(node, key, repValue); err != nil {
				log.Printf("Failed to write to replica %s: %v", node.ID, err)
				successChan <- false
			} else {
				successChan <- true
			}
		}(replica)
	}

	// Wait for writes to complete
	go func() {
		wg.Wait()
		close(successChan)
	}()

	// Count successes
	successCount := 0
	for success := range successChan {
		if success {
			successCount++
		}

		// Check if quorum reached
		if successCount >= r.config.WriteQuorum {
			return vc, nil
		}
	}

	return nil, fmt.Errorf("write quorum not reached: %d/%d", successCount, r.config.WriteQuorum)
}

// Read performs a quorum read from replicas
func (r *Replicator) Read(key string) ([]byte, *consistency.VectorClock, error) {
	// Get replica nodes
	replicas, err := r.getReplicaNodes(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get replicas: %v", err)
	}

	// Read from replicas
	type readResult struct {
		value *ReplicatedValue
		node  string
		err   error
	}

	resultChan := make(chan readResult, len(replicas))

	for _, replica := range replicas {
		go func(node *cluster.Node) {
			value, err := r.readFromNode(node, key)
			resultChan <- readResult{
				value: value,
				node:  node.ID,
				err:   err,
			}
		}(replica)
	}

	// Collect results
	var results []readResult
	timeout := time.NewTimer(r.config.Timeout)
	defer timeout.Stop()

	for i := 0; i < len(replicas); i++ {
		select {
		case result := <-resultChan:
			if result.err == nil && result.value != nil {
				results = append(results, result)
			}

			// Check if quorum reached
			if len(results) >= r.config.ReadQuorum {
				// Find the most recent value
				latest := r.findLatestValue(results)

				// Trigger read repair if needed
				go r.repairIfNeeded(key, results, latest)

				return latest.value.Data, latest.value.VectorClock, nil
			}

		case <-timeout.C:
			break
		}
	}

	return nil, nil, fmt.Errorf("read quorum not reached: %d/%d", len(results), r.config.ReadQuorum)
}

// getReplicaNodes returns N nodes responsible for the key
func (r *Replicator) getReplicaNodes(key string) ([]*cluster.Node, error) {
	nodes, err := r.nodeManager.GetNodesForKey(key, r.config.ReplicationFactor)
	if err != nil {
		return nil, err
	}

	if len(nodes) < r.config.ReplicationFactor {
		log.Printf("Warning: Only %d replicas available, wanted %d",
			len(nodes), r.config.ReplicationFactor)
	}

	return nodes, nil
}

// writeToNode writes data to a specific node
func (r *Replicator) writeToNode(node *cluster.Node, key string, value *ReplicatedValue) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/replicate/%s", node.Address, key)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Replication", "true")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("replication failed: status %d", resp.StatusCode)
	}

	return nil
}

// readFromNode reads data from a specific node
func (r *Replicator) readFromNode(node *cluster.Node, key string) (*ReplicatedValue, error) {
	url := fmt.Sprintf("http://%s/replicate/%s", node.Address, key)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Replication", "true")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read failed: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var value ReplicatedValue
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	return &value, nil
}

// findLatestValue finds the most recent value based on vector clocks
func (r *Replicator) findLatestValue(results []readResult) readResult {
	if len(results) == 0 {
		return readResult{}
	}

	latest := results[0]

	for i := 1; i < len(results); i++ {
		comparison := latest.value.VectorClock.Compare(results[i].value.VectorClock)

		switch comparison {
		case consistency.Before:
			latest = results[i]
		case consistency.Concurrent:
			// Use timestamp as tiebreaker (Last-Write-Wins)
			if results[i].value.Timestamp.After(latest.value.Timestamp) {
				latest = results[i]
			}
		}
	}

	return latest
}

// repairIfNeeded triggers read repair if inconsistencies detected
func (r *Replicator) repairIfNeeded(key string, results []readResult, latest readResult) {
	for _, result := range results {
		if result.node == latest.node {
			continue
		}

		comparison := result.value.VectorClock.Compare(latest.value.VectorClock)
		if comparison == consistency.Before {
			// This replica is outdated, repair it
			log.Printf("Read repair: updating stale replica %s for key %s", result.node, key)

			node, exists := r.nodeManager.GetNode(result.node)
			if exists {
				if err := r.writeToNode(node, key, latest.value); err != nil {
					log.Printf("Read repair failed for node %s: %v", result.node, err)
				}
			}
		}
	}
}

// Delete removes a key from all replicas
func (r *Replicator) Delete(key string) error {
	replicas, err := r.getReplicaNodes(key)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for _, replica := range replicas {
		wg.Add(1)
		go func(node *cluster.Node) {
			defer wg.Done()

			url := fmt.Sprintf("http://%s/replicate/%s", node.Address, key)
			req, err := http.NewRequest(http.MethodDelete, url, nil)
			if err != nil {
				return
			}

			req.Header.Set("X-Replication", "true")

			resp, err := r.httpClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(replica)
	}

	wg.Wait()

	if successCount < r.config.WriteQuorum {
		return fmt.Errorf("delete quorum not reached: %d/%d", successCount, r.config.WriteQuorum)
	}

	return nil
}
