package repair

import (
	"log"
	"sync"
	"time"
	
	"github.com/sjohri/dcache/internal/consistency"
	"github.com/sjohri/dcache/internal/replication"
)

// ReadRepair handles read repair operations
type ReadRepair struct {
	replicator *replication.Replicator
	enabled    bool
	stats      *RepairStats
	mu         sync.RWMutex
}

// RepairStats tracks read repair statistics
type RepairStats struct {
	RepairsPerformed   int64
	RepairsSucceeded   int64
	RepairsFailed      int64
	LastRepairTime     time.Time
	AverageRepairTime  time.Duration
}

// NewReadRepair creates a new read repair handler
func NewReadRepair(replicator *replication.Replicator, enabled bool) *ReadRepair {
	return &ReadRepair{
		replicator: replicator,
		enabled:    enabled,
		stats:      &RepairStats{},
	}
}

// RepairOnRead performs read repair when inconsistencies are detected
func (rr *ReadRepair) RepairOnRead(
	key string,
	versions []replication.ReplicatedValue,
	nodeIDs []string,
) error {
	if !rr.enabled {
		return nil
	}
	
	startTime := time.Now()
	
	// Find the most recent version using vector clocks
	latestIdx, conflicts := rr.findLatestVersion(versions)
	if latestIdx < 0 {
		return nil // No valid versions
	}
	
	latest := versions[latestIdx]
	
	// Track statistics
	defer func() {
		rr.updateStats(time.Since(startTime))
	}()
	
	// Repair all outdated replicas
	var wg sync.WaitGroup
	repairCount := 0
	successCount := 0
	
	for i, version := range versions {
		if i == latestIdx {
			continue // Skip the latest version
		}
		
		// Check if this version needs repair
		ordering := version.VectorClock.Compare(latest.VectorClock)
		if ordering == consistency.Before {
			repairCount++
			wg.Add(1)
			
			go func(nodeID string, outdatedVersion replication.ReplicatedValue) {
				defer wg.Done()
				
				log.Printf("Read repair: updating node %s for key %s", nodeID, key)
				
				// Update the outdated replica
				if err := rr.repairReplica(nodeID, key, latest); err != nil {
					log.Printf("Read repair failed for node %s: %v", nodeID, err)
					rr.recordFailure()
				} else {
					rr.recordSuccess()
					successCount++
				}
			}(nodeIDs[i], version)
		}
	}
	
	wg.Wait()
	
	// Handle conflicts if detected
	if len(conflicts) > 0 {
		log.Printf("Read repair: detected %d conflicts for key %s", len(conflicts), key)
		// Conflicts would be handled based on resolution strategy
	}
	
	if repairCount > 0 {
		log.Printf("Read repair: repaired %d/%d replicas for key %s", 
			successCount, repairCount, key)
	}
	
	return nil
}

// findLatestVersion finds the most recent version and detects conflicts
func (rr *ReadRepair) findLatestVersion(
	versions []replication.ReplicatedValue,
) (int, []int) {
	if len(versions) == 0 {
		return -1, nil
	}
	
	latestIdx := 0
	var conflicts []int
	
	for i := 1; i < len(versions); i++ {
		ordering := versions[latestIdx].VectorClock.Compare(versions[i].VectorClock)
		
		switch ordering {
		case consistency.Before:
			// Current version is newer
			latestIdx = i
			
		case consistency.Concurrent:
			// Conflict detected
			conflicts = append(conflicts, i)
			
			// Use timestamp as tiebreaker (LWW)
			if versions[i].Timestamp.After(versions[latestIdx].Timestamp) {
				latestIdx = i
			}
		}
	}
	
	return latestIdx, conflicts
}

// repairReplica updates an outdated replica
func (rr *ReadRepair) repairReplica(
	nodeID string,
	key string,
	value replication.ReplicatedValue,
) error {
	// This would send the updated value to the specific node
	// Implementation depends on the transport mechanism
	
	// For now, we'll use the replicator's write mechanism
	// In practice, this would be a direct node-to-node transfer
	
	return nil
}

// Enable enables read repair
func (rr *ReadRepair) Enable() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.enabled = true
}

// Disable disables read repair
func (rr *ReadRepair) Disable() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.enabled = false
}

// IsEnabled returns whether read repair is enabled
func (rr *ReadRepair) IsEnabled() bool {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return rr.enabled
}

// GetStats returns repair statistics
func (rr *ReadRepair) GetStats() RepairStats {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	return *rr.stats
}

// updateStats updates repair statistics
func (rr *ReadRepair) updateStats(duration time.Duration) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	
	rr.stats.RepairsPerformed++
	rr.stats.LastRepairTime = time.Now()
	
	// Update average repair time
	if rr.stats.AverageRepairTime == 0 {
		rr.stats.AverageRepairTime = duration
	} else {
		// Running average
		rr.stats.AverageRepairTime = (rr.stats.AverageRepairTime + duration) / 2
	}
}

// recordSuccess records a successful repair
func (rr *ReadRepair) recordSuccess() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.stats.RepairsSucceeded++
}

// recordFailure records a failed repair
func (rr *ReadRepair) recordFailure() {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.stats.RepairsFailed++
}
