package consistency

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
	
	"github.com/sjohri/dcache/internal/cluster"
)

// AntiEntropy implements periodic synchronization between replicas
type AntiEntropy struct {
	nodeManager *cluster.NodeManager
	merkleTree  *MerkleTree
	httpClient  *http.Client
	interval    time.Duration
	stopChan    chan struct{}
	mu          sync.RWMutex
}

// NewAntiEntropy creates a new anti-entropy protocol
func NewAntiEntropy(nodeManager *cluster.NodeManager, interval time.Duration) *AntiEntropy {
	return &AntiEntropy{
		nodeManager: nodeManager,
		merkleTree:  NewMerkleTree(),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start begins the anti-entropy process
func (ae *AntiEntropy) Start() {
	go ae.runAntiEntropy()
}

// Stop stops the anti-entropy process
func (ae *AntiEntropy) Stop() {
	close(ae.stopChan)
}

// runAntiEntropy periodically syncs with random peers
func (ae *AntiEntropy) runAntiEntropy() {
	ticker := time.NewTicker(ae.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ae.syncWithRandomPeer()
		case <-ae.stopChan:
			return
		}
	}
}

// syncWithRandomPeer selects a random peer and syncs
func (ae *AntiEntropy) syncWithRandomPeer() {
	nodes := ae.nodeManager.GetHealthyNodes()
	if len(nodes) <= 1 {
		return // No other nodes to sync with
	}
	
	// Select random peer (excluding self)
	var peer *cluster.Node
	for _, node := range nodes {
		if !ae.nodeManager.IsLocalNode(node.ID) {
			if rand.Float32() < 0.5 || peer == nil {
				peer = node
			}
		}
	}
	
	if peer == nil {
		return
	}
	
	log.Printf("Anti-entropy: syncing with peer %s", peer.ID)
	
	// Exchange merkle trees
	if err := ae.exchangeMerkleTrees(peer); err != nil {
		log.Printf("Anti-entropy sync failed with %s: %v", peer.ID, err)
		return
	}
}

// exchangeMerkleTrees exchanges and compares merkle trees
func (ae *AntiEntropy) exchangeMerkleTrees(peer *cluster.Node) error {
	// Get local merkle tree
	localTree := ae.getLocalMerkleTree()
	
	// Send local tree to peer and get peer's tree
	peerTree, err := ae.sendMerkleTree(peer, localTree)
	if err != nil {
		return fmt.Errorf("failed to exchange merkle trees: %v", err)
	}
	
	// Compare trees to find differences
	differences := localTree.Compare(peerTree)
	
	if len(differences) == 0 {
		log.Printf("Anti-entropy: no differences with peer %s", peer.ID)
		return nil
	}
	
	log.Printf("Anti-entropy: found %d differences with peer %s", len(differences), peer.ID)
	
	// Sync differences
	return ae.syncDifferences(peer, differences)
}

// getLocalMerkleTree builds a merkle tree of local data
func (ae *AntiEntropy) getLocalMerkleTree() *MerkleTree {
	ae.mu.RLock()
	defer ae.mu.RUnlock()
	
	// This would interact with the cache to build the tree
	// For now, return the stored tree
	return ae.merkleTree
}

// sendMerkleTree sends local tree to peer and receives peer's tree
func (ae *AntiEntropy) sendMerkleTree(peer *cluster.Node, localTree *MerkleTree) (*MerkleTree, error) {
	data, err := json.Marshal(localTree)
	if err != nil {
		return nil, err
	}
	
	url := fmt.Sprintf("http://%s/anti-entropy/exchange", peer.Address)
	resp, err := ae.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exchange failed: status %d", resp.StatusCode)
	}
	
	var peerTree MerkleTree
	if err := json.NewDecoder(resp.Body).Decode(&peerTree); err != nil {
		return nil, err
	}
	
	return &peerTree, nil
}

// syncDifferences synchronizes the differing keys
func (ae *AntiEntropy) syncDifferences(peer *cluster.Node, differences []string) error {
	// Request missing or outdated keys from peer
	for _, key := range differences {
		if err := ae.syncKey(peer, key); err != nil {
			log.Printf("Failed to sync key %s: %v", key, err)
			// Continue with other keys
		}
	}
	
	return nil
}

// syncKey synchronizes a single key with a peer
func (ae *AntiEntropy) syncKey(peer *cluster.Node, key string) error {
	// This would:
	// 1. Get the value and vector clock from peer
	// 2. Compare with local version
	// 3. Resolve conflicts if needed
	// 4. Update local or remote as appropriate
	
	url := fmt.Sprintf("http://%s/anti-entropy/sync/%s", peer.Address, key)
	resp, err := ae.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// Process the response and update local data if needed
	// This is simplified - actual implementation would handle vector clocks
	
	return nil
}

// UpdateMerkleTree updates the merkle tree when data changes
func (ae *AntiEntropy) UpdateMerkleTree(key string, value []byte) {
	ae.mu.Lock()
	defer ae.mu.Unlock()
	
	ae.merkleTree.Update(key, value)
}
