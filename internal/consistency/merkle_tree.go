package consistency

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
)

// MerkleTree implements a merkle tree for efficient data comparison
type MerkleTree struct {
	root     *MerkleNode
	leaves   map[string]*MerkleNode
	mu       sync.RWMutex
}

// MerkleNode represents a node in the merkle tree
type MerkleNode struct {
	Hash  string
	Key   string
	Left  *MerkleNode
	Right *MerkleNode
	IsLeaf bool
}

// NewMerkleTree creates a new merkle tree
func NewMerkleTree() *MerkleTree {
	return &MerkleTree{
		leaves: make(map[string]*MerkleNode),
	}
}

// Update adds or updates a key-value pair in the tree
func (mt *MerkleTree) Update(key string, value []byte) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	
	// Create or update leaf node
	hash := mt.hashData(append([]byte(key), value...))
	leaf := &MerkleNode{
		Hash:   hash,
		Key:    key,
		IsLeaf: true,
	}
	
	mt.leaves[key] = leaf
	mt.rebuild()
}

// Delete removes a key from the tree
func (mt *MerkleTree) Delete(key string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	
	delete(mt.leaves, key)
	mt.rebuild()
}

// rebuild reconstructs the merkle tree from leaves
func (mt *MerkleTree) rebuild() {
	if len(mt.leaves) == 0 {
		mt.root = nil
		return
	}
	
	// Sort keys for deterministic tree structure
	var keys []string
	for key := range mt.leaves {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	
	// Create leaf nodes in sorted order
	var nodes []*MerkleNode
	for _, key := range keys {
		nodes = append(nodes, mt.leaves[key])
	}
	
	// Build tree bottom-up
	for len(nodes) > 1 {
		var newLevel []*MerkleNode
		
		for i := 0; i < len(nodes); i += 2 {
			if i+1 < len(nodes) {
				// Combine two nodes
				parent := &MerkleNode{
					Hash:  mt.hashNodes(nodes[i].Hash, nodes[i+1].Hash),
					Left:  nodes[i],
					Right: nodes[i+1],
				}
				newLevel = append(newLevel, parent)
			} else {
				// Odd node, promote to next level
				newLevel = append(newLevel, nodes[i])
			}
		}
		
		nodes = newLevel
	}
	
	mt.root = nodes[0]
}

// GetRootHash returns the root hash of the tree
func (mt *MerkleTree) GetRootHash() string {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	
	if mt.root == nil {
		return ""
	}
	return mt.root.Hash
}

// Compare compares with another merkle tree and returns different keys
func (mt *MerkleTree) Compare(other *MerkleTree) []string {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	
	other.mu.RLock()
	defer other.mu.RUnlock()
	
	// Quick check: if root hashes match, trees are identical
	if mt.GetRootHash() == other.GetRootHash() {
		return nil
	}
	
	// Find differences by comparing nodes
	var differences []string
	mt.compareNodes(mt.root, other.root, &differences)
	
	return differences
}

// compareNodes recursively compares nodes to find differences
func (mt *MerkleTree) compareNodes(node1, node2 *MerkleNode, differences *[]string) {
	if node1 == nil && node2 == nil {
		return
	}
	
	if node1 == nil || node2 == nil || node1.Hash != node2.Hash {
		// Nodes differ, collect all keys under these nodes
		mt.collectKeys(node1, differences)
		mt.collectKeys(node2, differences)
		return
	}
	
	// Hashes match, recurse if not leaves
	if !node1.IsLeaf {
		mt.compareNodes(node1.Left, node2.Left, differences)
		mt.compareNodes(node1.Right, node2.Right, differences)
	}
}

// collectKeys collects all keys under a node
func (mt *MerkleTree) collectKeys(node *MerkleNode, keys *[]string) {
	if node == nil {
		return
	}
	
	if node.IsLeaf {
		*keys = append(*keys, node.Key)
		return
	}
	
	mt.collectKeys(node.Left, keys)
	mt.collectKeys(node.Right, keys)
}

// hashData computes SHA256 hash of data
func (mt *MerkleTree) hashData(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// hashNodes computes hash of two node hashes
func (mt *MerkleTree) hashNodes(hash1, hash2 string) string {
	combined := hash1 + hash2
	return mt.hashData([]byte(combined))
}

// GetKeys returns all keys in the tree
func (mt *MerkleTree) GetKeys() []string {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	
	var keys []string
	for key := range mt.leaves {
		keys = append(keys, key)
	}
	
	return keys
}

// Size returns the number of keys in the tree
func (mt *MerkleTree) Size() int {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	
	return len(mt.leaves)
}
