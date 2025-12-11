package consistency

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Ordering represents the relationship between vector clocks
type Ordering int

const (
	Equal Ordering = iota
	Before
	After
	Concurrent
)

// VectorClock implements Lamport vector clocks for causality tracking
type VectorClock struct {
	clock map[string]int
	mu    sync.RWMutex
}

// NewVectorClock creates a new vector clock
func NewVectorClock() *VectorClock {
	return &VectorClock{
		clock: make(map[string]int),
	}
}

// Increment increments the clock for a node
func (vc *VectorClock) Increment(nodeID string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	vc.clock[nodeID]++
}

// Update updates the clock with another vector clock
func (vc *VectorClock) Update(other *VectorClock) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	other.mu.RLock()
	defer other.mu.RUnlock()
	
	for nodeID, timestamp := range other.clock {
		if timestamp > vc.clock[nodeID] {
			vc.clock[nodeID] = timestamp
		}
	}
}

// Merge merges another vector clock and increments local node
func (vc *VectorClock) Merge(other *VectorClock, localNodeID string) {
	vc.Update(other)
	vc.Increment(localNodeID)
}

// Compare compares two vector clocks
func (vc *VectorClock) Compare(other *VectorClock) Ordering {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	other.mu.RLock()
	defer other.mu.RUnlock()
	
	var isLess, isGreater bool
	
	// Check all nodes in both clocks
	allNodes := make(map[string]bool)
	for node := range vc.clock {
		allNodes[node] = true
	}
	for node := range other.clock {
		allNodes[node] = true
	}
	
	for node := range allNodes {
		vcTime := vc.clock[node]
		otherTime := other.clock[node]
		
		if vcTime < otherTime {
			isLess = true
		} else if vcTime > otherTime {
			isGreater = true
		}
		
		// If both less and greater, clocks are concurrent
		if isLess && isGreater {
			return Concurrent
		}
	}
	
	if isLess && !isGreater {
		return Before
	} else if isGreater && !isLess {
		return After
	}
	
	return Equal
}

// Copy creates a deep copy of the vector clock
func (vc *VectorClock) Copy() *VectorClock {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	newClock := NewVectorClock()
	for node, timestamp := range vc.clock {
		newClock.clock[node] = timestamp
	}
	
	return newClock
}

// Get returns the timestamp for a node
func (vc *VectorClock) Get(nodeID string) int {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	return vc.clock[nodeID]
}

// String returns a string representation
func (vc *VectorClock) String() string {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	return fmt.Sprintf("%v", vc.clock)
}

// MarshalJSON implements json.Marshaler
func (vc *VectorClock) MarshalJSON() ([]byte, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	return json.Marshal(vc.clock)
}

// UnmarshalJSON implements json.Unmarshaler
func (vc *VectorClock) UnmarshalJSON(data []byte) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	
	return json.Unmarshal(data, &vc.clock)
}

// Size returns the number of nodes in the clock
func (vc *VectorClock) Size() int {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	
	return len(vc.clock)
}

// IsEmpty returns true if the clock is empty
func (vc *VectorClock) IsEmpty() bool {
	return vc.Size() == 0
}
