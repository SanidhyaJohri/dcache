package replication

import (
	"fmt"
	"time"
)

// QuorumManager handles quorum-based operations
type QuorumManager struct {
	replicationFactor int
	writeQuorum      int
	readQuorum       int
	timeout          time.Duration
}

// NewQuorumManager creates a new quorum manager
func NewQuorumManager(n, w, r int, timeout time.Duration) *QuorumManager {
	// Validate quorum settings
	if w+r <= n {
		panic("Invalid quorum: W+R must be > N for consistency")
	}
	
	return &QuorumManager{
		replicationFactor: n,
		writeQuorum:      w,
		readQuorum:       r,
		timeout:          timeout,
	}
}

// QuorumOperation represents an operation result
type QuorumOperation struct {
	NodeID  string
	Success bool
	Data    interface{}
	Error   error
}

// ExecuteWrite performs a quorum write operation
func (q *QuorumManager) ExecuteWrite(
	nodes []string,
	writeFunc func(nodeID string) error,
) error {
	if len(nodes) < q.writeQuorum {
		return fmt.Errorf("insufficient nodes: need %d, have %d", q.writeQuorum, len(nodes))
	}
	
	resultChan := make(chan QuorumOperation, len(nodes))
	
	// Execute writes in parallel
	for _, node := range nodes {
		go func(nodeID string) {
			err := writeFunc(nodeID)
			resultChan <- QuorumOperation{
				NodeID:  nodeID,
				Success: err == nil,
				Error:   err,
			}
		}(node)
	}
	
	// Collect results
	successCount := 0
	timeout := time.NewTimer(q.timeout)
	defer timeout.Stop()
	
	for i := 0; i < len(nodes); i++ {
		select {
		case result := <-resultChan:
			if result.Success {
				successCount++
				if successCount >= q.writeQuorum {
					return nil // Quorum reached
				}
			}
		case <-timeout.C:
			return fmt.Errorf("write timeout: only %d/%d succeeded", successCount, q.writeQuorum)
		}
	}
	
	return fmt.Errorf("write quorum not reached: %d/%d", successCount, q.writeQuorum)
}

// ExecuteRead performs a quorum read operation
func (q *QuorumManager) ExecuteRead(
	nodes []string,
	readFunc func(nodeID string) (interface{}, error),
) ([]QuorumOperation, error) {
	if len(nodes) < q.readQuorum {
		return nil, fmt.Errorf("insufficient nodes: need %d, have %d", q.readQuorum, len(nodes))
	}
	
	resultChan := make(chan QuorumOperation, len(nodes))
	
	// Execute reads in parallel
	for _, node := range nodes {
		go func(nodeID string) {
			data, err := readFunc(nodeID)
			resultChan <- QuorumOperation{
				NodeID:  nodeID,
				Success: err == nil,
				Data:    data,
				Error:   err,
			}
		}(node)
	}
	
	// Collect results
	var results []QuorumOperation
	timeout := time.NewTimer(q.timeout)
	defer timeout.Stop()
	
	for i := 0; i < len(nodes); i++ {
		select {
		case result := <-resultChan:
			if result.Success {
				results = append(results, result)
				if len(results) >= q.readQuorum {
					return results, nil // Quorum reached
				}
			}
		case <-timeout.C:
			return nil, fmt.Errorf("read timeout: only %d/%d succeeded", len(results), q.readQuorum)
		}
	}
	
	return nil, fmt.Errorf("read quorum not reached: %d/%d", len(results), q.readQuorum)
}

// IsQuorumPossible checks if quorum operations are possible with available nodes
func (q *QuorumManager) IsQuorumPossible(availableNodes int) (canRead, canWrite bool) {
	canRead = availableNodes >= q.readQuorum
	canWrite = availableNodes >= q.writeQuorum
	return
}
