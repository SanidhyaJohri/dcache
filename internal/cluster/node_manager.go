package cluster

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// NodeStatus represents the status of a node
type NodeStatus int

const (
	NodeHealthy NodeStatus = iota
	NodeSuspect
	NodeDead
)

// String returns string representation of NodeStatus
func (s NodeStatus) String() string {
	switch s {
	case NodeHealthy:
		return "healthy"
	case NodeSuspect:
		return "suspect"
	case NodeDead:
		return "dead"
	default:
		return "unknown"
	}
}

// Node represents a cluster node
type Node struct {
	ID          string
	Address     string
	Status      NodeStatus
	LastSeen    time.Time
	Metadata    map[string]string
}

// NodeManager manages cluster nodes
type NodeManager struct {
	localNodeID string
	nodes       map[string]*Node
	hashRing    *HashRing
	mu          sync.RWMutex
	
	// Callbacks
	onNodeJoin  func(nodeID string)
	onNodeLeave func(nodeID string)
}

// NewNodeManager creates a new node manager
func NewNodeManager(localNodeID string, virtualNodes int) *NodeManager {
	nm := &NodeManager{
		localNodeID: localNodeID,
		nodes:       make(map[string]*Node),
		hashRing:    NewHashRing(virtualNodes),
	}
	
	// Add local node
	nm.AddNode(&Node{
		ID:       localNodeID,
		Address:  "localhost:8080", // Will be updated by main
		Status:   NodeHealthy,
		LastSeen: time.Now(),
	})
	
	// Start health checker
	go nm.healthChecker()
	
	return nm
}

// AddNode adds a node to the cluster
func (nm *NodeManager) AddNode(node *Node) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	// Fix address if it's just the gossip port
	if node.Address == "" || strings.HasPrefix(node.Address, ":") {
		// Use node ID as hostname if address is missing
		node.Address = fmt.Sprintf("%s:8080", node.ID)
	}
	
	if _, exists := nm.nodes[node.ID]; exists {
		// Update existing node
		nm.nodes[node.ID].LastSeen = time.Now()
		nm.nodes[node.ID].Status = NodeHealthy
		// Update address if better one provided
		if node.Address != "" && !strings.HasPrefix(node.Address, ":") {
			nm.nodes[node.ID].Address = node.Address
		}
		return nil
	}
	
	nm.nodes[node.ID] = node
	nm.hashRing.AddNode(node.ID)
	
	log.Printf("Node %s joined the cluster at %s", node.ID, node.Address)
	
	if nm.onNodeJoin != nil {
		go nm.onNodeJoin(node.ID)
	}
	
	return nil
}

// UpdateNodeAddress updates a node's address
func (nm *NodeManager) UpdateNodeAddress(nodeID, address string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	if node, exists := nm.nodes[nodeID]; exists {
		node.Address = address
		log.Printf("Updated node %s address to %s", nodeID, address)
	}
}

// RemoveNode removes a node from the cluster
func (nm *NodeManager) RemoveNode(nodeID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	if _, exists := nm.nodes[nodeID]; !exists {
		return
	}
	
	delete(nm.nodes, nodeID)
	nm.hashRing.RemoveNode(nodeID)
	
	log.Printf("Node %s left the cluster", nodeID)
	
	if nm.onNodeLeave != nil {
		go nm.onNodeLeave(nodeID)
	}
}

// GetNode returns a node by ID
func (nm *NodeManager) GetNode(nodeID string) (*Node, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	node, exists := nm.nodes[nodeID]
	return node, exists
}

// GetAllNodes returns all nodes in the cluster
func (nm *NodeManager) GetAllNodes() []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	nodes := make([]*Node, 0, len(nm.nodes))
	for _, node := range nm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetHealthyNodes returns only healthy nodes
func (nm *NodeManager) GetHealthyNodes() []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	nodes := make([]*Node, 0, len(nm.nodes))
	for _, node := range nm.nodes {
		if node.Status == NodeHealthy {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetNodeForKey returns the node responsible for a key
func (nm *NodeManager) GetNodeForKey(key string) (*Node, error) {
	nodeID := nm.hashRing.GetNode(key)
	if nodeID == "" {
		return nil, fmt.Errorf("no nodes available")
	}
	
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	node, exists := nm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}
	
	return node, nil
}

// GetNodesForKey returns N nodes for replication
func (nm *NodeManager) GetNodesForKey(key string, count int) ([]*Node, error) {
	nodeIDs := nm.hashRing.GetNodes(key, count)
	if len(nodeIDs) == 0 {
		return nil, fmt.Errorf("no nodes available")
	}
	
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	nodes := make([]*Node, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if node, exists := nm.nodes[nodeID]; exists && node.Status == NodeHealthy {
			nodes = append(nodes, node)
		}
	}
	
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no healthy nodes available")
	}
	
	return nodes, nil
}

// UpdateNodeStatus updates the status of a node
func (nm *NodeManager) UpdateNodeStatus(nodeID string, status NodeStatus) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	if node, exists := nm.nodes[nodeID]; exists {
		node.Status = status
		if status == NodeHealthy {
			node.LastSeen = time.Now()
		}
	}
}

// healthChecker periodically checks node health
func (nm *NodeManager) healthChecker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		nm.mu.Lock()
		now := time.Now()
		
		for nodeID, node := range nm.nodes {
			if nodeID == nm.localNodeID {
				continue // skip local node
			}
			
			timeSinceLastSeen := now.Sub(node.LastSeen)
			
			switch node.Status {
			case NodeHealthy:
				if timeSinceLastSeen > 10*time.Second {
					node.Status = NodeSuspect
					log.Printf("Node %s is now suspect", nodeID)
				}
			case NodeSuspect:
				if timeSinceLastSeen > 30*time.Second {
					node.Status = NodeDead
					log.Printf("Node %s is now dead", nodeID)
				}
			case NodeDead:
				if timeSinceLastSeen > 60*time.Second {
					// Remove dead node after 60 seconds
					delete(nm.nodes, nodeID)
					nm.hashRing.RemoveNode(nodeID)
					log.Printf("Node %s removed from cluster", nodeID)
				}
			}
		}
		
		nm.mu.Unlock()
	}
}

// SetOnNodeJoin sets callback for node join events
func (nm *NodeManager) SetOnNodeJoin(fn func(nodeID string)) {
	nm.onNodeJoin = fn
}

// SetOnNodeLeave sets callback for node leave events
func (nm *NodeManager) SetOnNodeLeave(fn func(nodeID string)) {
	nm.onNodeLeave = fn
}

// GetLocalNodeID returns the local node ID
func (nm *NodeManager) GetLocalNodeID() string {
	return nm.localNodeID
}

// IsLocalNode checks if a nodeID is the local node
func (nm *NodeManager) IsLocalNode(nodeID string) bool {
	return nodeID == nm.localNodeID
}

// GetHashRing returns the hash ring
func (nm *NodeManager) GetHashRing() *HashRing {
	return nm.hashRing
}
