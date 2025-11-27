package crdt

import (
	"encoding/json"
	"sync"
)

// VectorClock tracks causality between distributed events
// Maps node/agent ID to logical timestamp
type VectorClock struct {
	Clock map[string]int64 `json:"clock"`
	mu    sync.RWMutex
}

// NewVectorClock creates a new vector clock
func NewVectorClock() *VectorClock {
	return &VectorClock{
		Clock: make(map[string]int64),
	}
}

// Increment increases the clock value for a specific node
func (vc *VectorClock) Increment(nodeID string) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.Clock[nodeID]++
}

// Get returns the clock value for a specific node
func (vc *VectorClock) Get(nodeID string) int64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.Clock[nodeID]
}

// Set updates the clock value for a specific node
func (vc *VectorClock) Set(nodeID string, value int64) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.Clock[nodeID] = value
}

// Merge combines two vector clocks by taking the maximum for each node
// Used when receiving operations from other nodes
func (vc *VectorClock) Merge(other *VectorClock) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	for nodeID, timestamp := range other.Clock {
		if vc.Clock[nodeID] < timestamp {
			vc.Clock[nodeID] = timestamp
		}
	}
}

// CompareResult represents the relationship between two vector clocks
type CompareResult int

const (
	// Concurrent means the events are concurrent (no causal relationship)
	Concurrent CompareResult = iota
	// Before means this clock happened before the other
	Before
	// After means this clock happened after the other
	After
	// Equal means the clocks are identical
	Equal
)

// Compare determines the causal relationship between two vector clocks
func (vc *VectorClock) Compare(other *VectorClock) CompareResult {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	other.mu.RLock()
	defer other.mu.RUnlock()

	allEqual := true
	anyLess := false
	anyGreater := false

	// Collect all node IDs from both clocks
	allNodes := make(map[string]bool)
	for nodeID := range vc.Clock {
		allNodes[nodeID] = true
	}
	for nodeID := range other.Clock {
		allNodes[nodeID] = true
	}

	// Compare each node's timestamp
	for nodeID := range allNodes {
		thisTime := vc.Clock[nodeID]
		otherTime := other.Clock[nodeID]

		if thisTime < otherTime {
			anyLess = true
			allEqual = false
		} else if thisTime > otherTime {
			anyGreater = true
			allEqual = false
		}
	}

	if allEqual {
		return Equal
	}
	if anyLess && !anyGreater {
		return Before
	}
	if anyGreater && !anyLess {
		return After
	}
	return Concurrent
}

// HappenedBefore checks if this clock happened before the other
func (vc *VectorClock) HappenedBefore(other *VectorClock) bool {
	return vc.Compare(other) == Before
}

// HappenedAfter checks if this clock happened after the other
func (vc *VectorClock) HappenedAfter(other *VectorClock) bool {
	return vc.Compare(other) == After
}

// IsConcurrent checks if this clock is concurrent with the other
func (vc *VectorClock) IsConcurrent(other *VectorClock) bool {
	return vc.Compare(other) == Concurrent
}

// Copy creates a deep copy of the vector clock
func (vc *VectorClock) Copy() *VectorClock {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	newClock := NewVectorClock()
	for nodeID, timestamp := range vc.Clock {
		newClock.Clock[nodeID] = timestamp
	}
	return newClock
}

// MarshalJSON implements custom JSON marshaling
func (vc *VectorClock) MarshalJSON() ([]byte, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	return json.Marshal(vc.Clock)
}

// UnmarshalJSON implements custom JSON unmarshaling
func (vc *VectorClock) UnmarshalJSON(data []byte) error {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	if vc.Clock == nil {
		vc.Clock = make(map[string]int64)
	}

	return json.Unmarshal(data, &vc.Clock)
}
