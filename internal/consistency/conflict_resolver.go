package consistency

import (
	"time"
)

// ConflictResolutionStrategy defines how to resolve conflicts
type ConflictResolutionStrategy int

const (
	LastWriteWins ConflictResolutionStrategy = iota
	FirstWriteWins
	HighestValueWins
	MergeSiblings
	ClientResolution
)

// ConflictResolver handles conflict resolution for concurrent updates
type ConflictResolver struct {
	strategy ConflictResolutionStrategy
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(strategy ConflictResolutionStrategy) *ConflictResolver {
	return &ConflictResolver{
		strategy: strategy,
	}
}

// Version represents a version of a value
type Version struct {
	Data        []byte
	VectorClock *VectorClock
	Timestamp   time.Time
	NodeID      string
}

// Resolve resolves conflicts between multiple versions
func (cr *ConflictResolver) Resolve(versions []Version) (Version, []Version) {
	if len(versions) == 0 {
		return Version{}, nil
	}
	
	if len(versions) == 1 {
		return versions[0], nil
	}
	
	switch cr.strategy {
	case LastWriteWins:
		return cr.resolveLastWriteWins(versions)
		
	case FirstWriteWins:
		return cr.resolveFirstWriteWins(versions)
		
	case HighestValueWins:
		return cr.resolveHighestValueWins(versions)
		
	case MergeSiblings:
		return cr.resolveMergeSiblings(versions)
		
	case ClientResolution:
		// Return all versions for client to resolve
		return Version{}, versions
		
	default:
		// Default to LWW
		return cr.resolveLastWriteWins(versions)
	}
}

// resolveLastWriteWins picks the version with the latest timestamp
func (cr *ConflictResolver) resolveLastWriteWins(versions []Version) (Version, []Version) {
	latest := versions[0]
	
	for i := 1; i < len(versions); i++ {
		if versions[i].Timestamp.After(latest.Timestamp) {
			latest = versions[i]
		}
	}
	
	return latest, nil
}

// resolveFirstWriteWins picks the version with the earliest timestamp
func (cr *ConflictResolver) resolveFirstWriteWins(versions []Version) (Version, []Version) {
	earliest := versions[0]
	
	for i := 1; i < len(versions); i++ {
		if versions[i].Timestamp.Before(earliest.Timestamp) {
			earliest = versions[i]
		}
	}
	
	return earliest, nil
}

// resolveHighestValueWins picks the version with the highest value (lexicographically)
func (cr *ConflictResolver) resolveHighestValueWins(versions []Version) (Version, []Version) {
	highest := versions[0]
	
	for i := 1; i < len(versions); i++ {
		if string(versions[i].Data) > string(highest.Data) {
			highest = versions[i]
		}
	}
	
	return highest, nil
}

// resolveMergeSiblings merges concurrent versions into siblings
func (cr *ConflictResolver) resolveMergeSiblings(versions []Version) (Version, []Version) {
	// Find all concurrent versions (siblings)
	var siblings []Version
	
	for i := 0; i < len(versions); i++ {
		isConcurrent := false
		
		for j := 0; j < len(versions); j++ {
			if i == j {
				continue
			}
			
			ordering := versions[i].VectorClock.Compare(versions[j].VectorClock)
			if ordering == Concurrent {
				isConcurrent = true
				break
			}
		}
		
		if isConcurrent {
			siblings = append(siblings, versions[i])
		}
	}
	
	// If no concurrent versions, return the latest
	if len(siblings) == 0 {
		return cr.resolveLastWriteWins(versions)
	}
	
	// Return empty winner with siblings for application to handle
	return Version{}, siblings
}

// DetectConflicts identifies concurrent versions
func (cr *ConflictResolver) DetectConflicts(versions []Version) [][]Version {
	var conflictSets [][]Version
	used := make(map[int]bool)
	
	for i := 0; i < len(versions); i++ {
		if used[i] {
			continue
		}
		
		conflictSet := []Version{versions[i]}
		used[i] = true
		
		for j := i + 1; j < len(versions); j++ {
			if used[j] {
				continue
			}
			
			ordering := versions[i].VectorClock.Compare(versions[j].VectorClock)
			if ordering == Concurrent {
				conflictSet = append(conflictSet, versions[j])
				used[j] = true
			}
		}
		
		if len(conflictSet) > 1 {
			conflictSets = append(conflictSets, conflictSet)
		}
	}
	
	return conflictSets
}
