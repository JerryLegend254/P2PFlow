package analytics

import (
	"math"
	"sort"
	"time"
)

// FilePriority represents the priority level for bandwidth allocation
type FilePriority int

const (
	PriorityCritical FilePriority = 5
	PriorityHigh     FilePriority = 4
	PriorityMedium   FilePriority = 3
	PriorityLow      FilePriority = 2
	PriorityMinimal  FilePriority = 1
)

// BandwidthAllocation represents bandwidth allocation for a file
type BandwidthAllocation struct {
	FilePath       string       `json:"file_path"`
	Importance     float64      `json:"importance"`      // 0.0 to 1.0
	Priority       FilePriority `json:"priority"`
	AllocatedBytes int64        `json:"allocated_bytes"` // Bytes per second
	Reason         string       `json:"reason"`
	Timestamp      time.Time    `json:"timestamp"`
}

// BandwidthAllocator intelligently allocates bandwidth based on file importance
type BandwidthAllocator struct {
	tracker         *AccessTracker
	totalBandwidth  int64 // Total available bandwidth in bytes/second
	allocations     map[string]*BandwidthAllocation
	decayFactor     float64 // How quickly importance decays over time
}

// NewBandwidthAllocator creates a new bandwidth allocator
func NewBandwidthAllocator(tracker *AccessTracker) *BandwidthAllocator {
	return &BandwidthAllocator{
		tracker:        tracker,
		totalBandwidth: 1024 * 1024 * 10, // Default 10 MB/s
		allocations:    make(map[string]*BandwidthAllocation),
		decayFactor:    0.9, // 10% decay per day
	}
}

// SetTotalBandwidth sets the total available bandwidth
func (b *BandwidthAllocator) SetTotalBandwidth(bytesPerSecond int64) {
	b.totalBandwidth = bytesPerSecond
}

// GetFileImportance calculates the importance score for a file (0.0 to 1.0)
func (b *BandwidthAllocator) GetFileImportance(filePath string) float64 {
	fileStats := b.tracker.GetFileStatistics(filePath)
	if fileStats == nil {
		return 0.5 // Default medium importance
	}

	// Multiple factors contribute to importance:
	// 1. Access frequency
	// 2. Recency of access
	// 3. Number of different peers accessing
	// 4. Consistency of access pattern

	importance := 0.0

	// Factor 1: Access frequency (0-0.3)
	daysSinceFirst := time.Since(fileStats.FirstAccess).Hours() / 24
	if daysSinceFirst < 1 {
		daysSinceFirst = 1
	}
	accessesPerDay := float64(fileStats.TotalAccesses) / daysSinceFirst
	frequencyScore := math.Min(accessesPerDay/10.0, 1.0) * 0.3
	importance += frequencyScore

	// Factor 2: Recency (0-0.3)
	daysSinceLastAccess := time.Since(fileStats.LastAccess).Hours() / 24
	recencyScore := math.Exp(-daysSinceLastAccess) * 0.3 // Exponential decay
	importance += recencyScore

	// Factor 3: Peer diversity (0-0.2)
	peerCount := len(fileStats.PeerAccesses)
	peerScore := math.Min(float64(peerCount)/5.0, 1.0) * 0.2
	importance += peerScore

	// Factor 4: Consistency (0-0.2)
	// Files with regular access patterns are more important
	consistencyScore := b.calculateConsistency(fileStats) * 0.2
	importance += consistencyScore

	// Cap at 1.0
	return math.Min(importance, 1.0)
}

// calculateConsistency measures how consistent the access pattern is
func (b *BandwidthAllocator) calculateConsistency(fileStats *FileStatistics) float64 {
	if fileStats.TotalAccesses < 5 {
		return 0.5 // Not enough data
	}

	// Check hourly pattern consistency
	// Files accessed at consistent times have higher consistency
	maxHourCount := 0
	for _, count := range fileStats.HourlyPattern {
		if count > maxHourCount {
			maxHourCount = count
		}
	}

	// If most accesses happen at specific hours, it's consistent
	consistency := float64(maxHourCount) / float64(fileStats.TotalAccesses)

	// Also check day-of-week pattern
	maxDayCount := 0
	for _, count := range fileStats.DayOfWeekPattern {
		if count > maxDayCount {
			maxDayCount = count
		}
	}
	dayConsistency := float64(maxDayCount) / float64(fileStats.TotalAccesses)

	// Average the two
	return (consistency + dayConsistency) / 2
}

// AllocateBandwidth allocates bandwidth across files based on importance
func (b *BandwidthAllocator) AllocateBandwidth(filePaths []string) map[string]*BandwidthAllocation {
	if len(filePaths) == 0 {
		return make(map[string]*BandwidthAllocation)
	}

	// Calculate importance for each file
	type fileScore struct {
		path       string
		importance float64
	}

	scores := make([]fileScore, 0, len(filePaths))
	totalImportance := 0.0

	for _, path := range filePaths {
		importance := b.GetFileImportance(path)
		scores = append(scores, fileScore{path: path, importance: importance})
		totalImportance += importance
	}

	// Sort by importance (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].importance > scores[j].importance
	})

	// Allocate bandwidth proportionally
	allocations := make(map[string]*BandwidthAllocation)

	if totalImportance == 0 {
		// Equal allocation
		bytesPerFile := b.totalBandwidth / int64(len(filePaths))
		for _, path := range filePaths {
			allocations[path] = &BandwidthAllocation{
				FilePath:       path,
				Importance:     0.5,
				Priority:       PriorityMedium,
				AllocatedBytes: bytesPerFile,
				Reason:         "Equal distribution (no history)",
				Timestamp:      time.Now(),
			}
		}
	} else {
		// Proportional allocation with minimum guarantees
		minAllocation := b.totalBandwidth / 100 // 1% minimum

		for _, score := range scores {
			proportion := score.importance / totalImportance
			allocated := int64(float64(b.totalBandwidth) * proportion)

			// Ensure minimum allocation
			if allocated < minAllocation {
				allocated = minAllocation
			}

			priority := b.importanceToPriority(score.importance)
			reason := b.generateAllocationReason(score.path, score.importance)

			allocations[score.path] = &BandwidthAllocation{
				FilePath:       score.path,
				Importance:     score.importance,
				Priority:       priority,
				AllocatedBytes: allocated,
				Reason:         reason,
				Timestamp:      time.Now(),
			}
		}
	}

	b.allocations = allocations
	return allocations
}

// importanceToPriority converts importance score to priority level
func (b *BandwidthAllocator) importanceToPriority(importance float64) FilePriority {
	if importance >= 0.8 {
		return PriorityCritical
	} else if importance >= 0.6 {
		return PriorityHigh
	} else if importance >= 0.4 {
		return PriorityMedium
	} else if importance >= 0.2 {
		return PriorityLow
	}
	return PriorityMinimal
}

// generateAllocationReason generates a human-readable reason for the allocation
func (b *BandwidthAllocator) generateAllocationReason(filePath string, importance float64) string {
	fileStats := b.tracker.GetFileStatistics(filePath)
	if fileStats == nil {
		return "No access history"
	}

	if importance >= 0.8 {
		return "High importance: frequently accessed and recent"
	} else if importance >= 0.6 {
		return "Medium-high importance: regular access pattern"
	} else if importance >= 0.4 {
		return "Medium importance: moderate access frequency"
	} else if importance >= 0.2 {
		return "Low importance: infrequent access"
	}
	return "Minimal importance: rarely accessed"
}

// GetAllocation returns the bandwidth allocation for a specific file
func (b *BandwidthAllocator) GetAllocation(filePath string) *BandwidthAllocation {
	allocation, exists := b.allocations[filePath]
	if !exists {
		return nil
	}

	// Return a copy
	copy := *allocation
	return &copy
}

// GetAllAllocations returns all current bandwidth allocations
func (b *BandwidthAllocator) GetAllAllocations() []*BandwidthAllocation {
	allocations := make([]*BandwidthAllocation, 0, len(b.allocations))
	for _, allocation := range b.allocations {
		copy := *allocation
		allocations = append(allocations, &copy)
	}

	// Sort by importance (descending)
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].Importance > allocations[j].Importance
	})

	return allocations
}

// SuggestOptimalBandwidth suggests optimal bandwidth allocation for active transfers
func (b *BandwidthAllocator) SuggestOptimalBandwidth(activeFiles []string) map[string]int64 {
	if len(activeFiles) == 0 {
		return make(map[string]int64)
	}

	// Calculate importance-weighted allocation
	allocations := b.AllocateBandwidth(activeFiles)

	// Convert to simple map
	result := make(map[string]int64)
	for path, allocation := range allocations {
		result[path] = allocation.AllocatedBytes
	}

	return result
}

// RebalanceBandwidth redistributes bandwidth based on current activity
func (b *BandwidthAllocator) RebalanceBandwidth(activeFiles []string, usageStats map[string]int64) {
	// Reallocate based on current usage and importance
	b.AllocateBandwidth(activeFiles)

	// Adjust allocations based on actual usage patterns
	for filePath, currentUsage := range usageStats {
		allocation, exists := b.allocations[filePath]
		if !exists {
			continue
		}

		// If a file is using significantly less than allocated, redistribute
		if currentUsage < allocation.AllocatedBytes/2 {
			// File is underutilizing, reduce allocation
			allocation.AllocatedBytes = currentUsage + (allocation.AllocatedBytes-currentUsage)/2
		}
	}
}

// GetPriorityFiles returns files above a certain priority level
func (b *BandwidthAllocator) GetPriorityFiles(minPriority FilePriority) []string {
	files := make([]string, 0)

	for _, allocation := range b.allocations {
		if allocation.Priority >= minPriority {
			files = append(files, allocation.FilePath)
		}
	}

	return files
}

// EstimateTransferTime estimates how long a file transfer will take
func (b *BandwidthAllocator) EstimateTransferTime(filePath string, sizeBytes int64) time.Duration {
	importance := b.GetFileImportance(filePath)
	priority := b.importanceToPriority(importance)

	// Estimate bandwidth based on priority
	var estimatedBandwidth int64
	switch priority {
	case PriorityCritical:
		estimatedBandwidth = b.totalBandwidth / 2 // 50% for critical
	case PriorityHigh:
		estimatedBandwidth = b.totalBandwidth / 4 // 25% for high
	case PriorityMedium:
		estimatedBandwidth = b.totalBandwidth / 8 // 12.5% for medium
	case PriorityLow:
		estimatedBandwidth = b.totalBandwidth / 16 // 6.25% for low
	default:
		estimatedBandwidth = b.totalBandwidth / 32 // 3.125% for minimal
	}

	if estimatedBandwidth == 0 {
		estimatedBandwidth = 1024 * 100 // Fallback to 100 KB/s
	}

	seconds := sizeBytes / estimatedBandwidth
	return time.Duration(seconds) * time.Second
}
