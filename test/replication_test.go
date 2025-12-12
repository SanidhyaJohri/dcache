package test

import (
	"testing"
	"time"
	
	"github.com/sjohri/dcache/internal/consistency"
	"github.com/sjohri/dcache/internal/replication"
)

func TestVectorClock(t *testing.T) {
	vc1 := consistency.NewVectorClock()
	vc2 := consistency.NewVectorClock()
	
	// Test increment
	vc1.Increment("node1")
	vc1.Increment("node1")
	vc2.Increment("node2")
	
	// Test comparison
	if vc1.Compare(vc2) != consistency.Concurrent {
		t.Error("Clocks should be concurrent")
	}
	
	// Test merge
	vc1.Update(vc2)
	vc1.Increment("node1")
	
	if vc1.Compare(vc2) != consistency.After {
		t.Error("vc1 should be after vc2")
	}
}

func TestQuorumOperations(t *testing.T) {
	qm := replication.NewQuorumManager(3, 2, 2, 5*time.Second)
	
	// Test write quorum
	nodes := []string{"node1", "node2", "node3"}
	successCount := 0
	
	err := qm.ExecuteWrite(nodes, func(nodeID string) error {
		successCount++
		if successCount >= 2 {
			return nil // Simulate success
		}
		return fmt.Errorf("simulated failure")
	})
	
	if err != nil {
		t.Errorf("Write quorum should succeed with 2/3 nodes")
	}
}

func TestConflictResolution(t *testing.T) {
	resolver := consistency.NewConflictResolver(consistency.LastWriteWins)
	
	// Create conflicting versions
	vc1 := consistency.NewVectorClock()
	vc1.Increment("node1")
	
	vc2 := consistency.NewVectorClock()
	vc2.Increment("node2")
	
	versions := []consistency.Version{
		{
			Data:        []byte("value1"),
			VectorClock: vc1,
			Timestamp:   time.Now(),
		},
		{
			Data:        []byte("value2"),
			VectorClock: vc2,
			Timestamp:   time.Now().Add(1 * time.Second),
		},
	}
	
	// Resolve conflict
	winner, siblings := resolver.Resolve(versions)
	
	if winner.Data == nil {
		t.Error("Should have resolved to a winner")
	}
	
	if len(siblings) != 0 {
		t.Error("LWW should not return siblings")
	}
}

func TestMerkleTree(t *testing.T) {
	tree1 := consistency.NewMerkleTree()
	tree2 := consistency.NewMerkleTree()
	
	// Add same data to both trees
	tree1.Update("key1", []byte("value1"))
	tree1.Update("key2", []byte("value2"))
	
	tree2.Update("key1", []byte("value1"))
	tree2.Update("key2", []byte("value2"))
	
	// Trees should be identical
	if tree1.GetRootHash() != tree2.GetRootHash() {
		t.Error("Trees with same data should have same root hash")
	}
	
	// Add different data
	tree2.Update("key3", []byte("value3"))
	
	// Trees should differ
	differences := tree1.Compare(tree2)
	if len(differences) == 0 {
		t.Error("Should detect differences")
	}
}
