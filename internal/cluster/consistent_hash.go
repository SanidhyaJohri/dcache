package cluster

import (
	"crypto/md5"
	"fmt"
	"sort"
	"sync"
)

// HashRing represents a consistent hash ring with virtual nodes
type HashRing struct {
	nodes        map[uint32]string // hash value -> node ID
	sortedHashes []uint32         // sorted hash values for binary search
	virtualNodes int              // number of virtual nodes per physical node
	nodeSet      map[string]bool  // set of physical nodes
	mu           sync.RWMutex
}

// NewHashRing creates a new consistent hash ring
func NewHashRing(virtualNodes int) *HashRing {
	if virtualNodes < 1 {
		virtualNodes = 150 // default virtual nodes
	}
	return &HashRing{
		nodes:        make(map[uint32]string),
		virtualNodes: virtualNodes,
		nodeSet:      make(map[string]bool),
	}
}

// AddNode adds a node to the hash ring with virtual nodes
func (h *HashRing) AddNode(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.nodeSet[nodeID] {
		return // node already exists
	}
	
	h.nodeSet[nodeID] = true
	
	// Add virtual nodes
	for i := 0; i < h.virtualNodes; i++ {
		virtualKey := fmt.Sprintf("%s:%d", nodeID, i)
		hash := h.hash(virtualKey)
		h.nodes[hash] = nodeID
		h.sortedHashes = append(h.sortedHashes, hash)
	}
	
	// Re-sort the hash values
	sort.Slice(h.sortedHashes, func(i, j int) bool {
		return h.sortedHashes[i] < h.sortedHashes[j]
	})
}

// RemoveNode removes a node from the hash ring
func (h *HashRing) RemoveNode(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.nodeSet[nodeID] {
		return // node doesn't exist
	}
	
	delete(h.nodeSet, nodeID)
	
	// Remove virtual nodes
	for i := 0; i < h.virtualNodes; i++ {
		virtualKey := fmt.Sprintf("%s:%d", nodeID, i)
		hash := h.hash(virtualKey)
		delete(h.nodes, hash)
	}
	
	// Rebuild sorted hashes
	h.sortedHashes = h.sortedHashes[:0]
	for hash := range h.nodes {
		h.sortedHashes = append(h.sortedHashes, hash)
	}
	
	sort.Slice(h.sortedHashes, func(i, j int) bool {
		return h.sortedHashes[i] < h.sortedHashes[j]
	})
}

// GetNode returns the node responsible for the given key
func (h *HashRing) GetNode(key string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if len(h.sortedHashes) == 0 {
		return ""
	}
	
	hash := h.hash(key)
	
	// Binary search for the first node with hash >= key hash
	idx := sort.Search(len(h.sortedHashes), func(i int) bool {
		return h.sortedHashes[i] >= hash
	})
	
	// Wrap around if necessary
	if idx == len(h.sortedHashes) {
		idx = 0
	}
	
	return h.nodes[h.sortedHashes[idx]]
}

// GetNodes returns N nodes for replication
func (h *HashRing) GetNodes(key string, count int) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if len(h.sortedHashes) == 0 {
		return nil
	}
	
	if count > len(h.nodeSet) {
		count = len(h.nodeSet)
	}
	
	hash := h.hash(key)
	nodes := make([]string, 0, count)
	seen := make(map[string]bool)
	
	// Find starting position
	idx := sort.Search(len(h.sortedHashes), func(i int) bool {
		return h.sortedHashes[i] >= hash
	})
	
	// Collect unique nodes
	for i := 0; i < len(h.sortedHashes) && len(nodes) < count; i++ {
		actualIdx := (idx + i) % len(h.sortedHashes)
		node := h.nodes[h.sortedHashes[actualIdx]]
		
		if !seen[node] {
			seen[node] = true
			nodes = append(nodes, node)
		}
	}
	
	return nodes
}

// GetAllNodes returns all physical nodes in the ring
func (h *HashRing) GetAllNodes() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	nodes := make([]string, 0, len(h.nodeSet))
	for node := range h.nodeSet {
		nodes = append(nodes, node)
	}
	return nodes
}

// IsEmpty returns true if the ring has no nodes
func (h *HashRing) IsEmpty() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodeSet) == 0
}

// hash generates a 32-bit hash for a string
func (h *HashRing) hash(key string) uint32 {
	hash := md5.Sum([]byte(key))
	return uint32(hash[0])<<24 | uint32(hash[1])<<16 | uint32(hash[2])<<8 | uint32(hash[3])
}

// Stats returns statistics about key distribution
func (h *HashRing) Stats() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	stats := make(map[string]int)
	for _, node := range h.nodes {
		stats[node]++
	}
	return stats
}
