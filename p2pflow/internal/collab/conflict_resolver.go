package collab

import (
	"fmt"
	"sort"
	"strings"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

// ConflictType represents the type of conflict
type ConflictType int

const (
	NoConflict ConflictType = iota
	OverlappingEdit
	SimultaneousEdit
	ContentConflict
)

// Conflict represents a merge conflict
type Conflict struct {
	Type        ConflictType `json:"type"`
	Description string       `json:"description"`
	Agent1      string       `json:"agent1"`
	Agent2      string       `json:"agent2"`
	Position    int          `json:"position"`
	Resolution  string       `json:"resolution,omitempty"`
}

// ConflictResolver handles conflict detection and resolution
type ConflictResolver struct {
	dmp *dmp.DiffMatchPatch
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver() *ConflictResolver {
	return &ConflictResolver{
		dmp: dmp.New(),
	}
}

// DetectConflicts analyzes changes for potential conflicts
func (cr *ConflictResolver) DetectConflicts(changes []*ChangeEvent) []*Conflict {
	var conflicts []*Conflict

	// Sort changes by timestamp
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Timestamp.Before(changes[j].Timestamp)
	})

	// Check for overlapping edits
	for i := 0; i < len(changes); i++ {
		for j := i + 1; j < len(changes); j++ {
			conflict := cr.analyzeChanges(changes[i], changes[j])
			if conflict != nil {
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts
}

// analyzeChanges compares two changes for conflicts
func (cr *ConflictResolver) analyzeChanges(change1, change2 *ChangeEvent) *Conflict {
	// Parse patches
	patches1, err1 := cr.dmp.PatchFromText(change1.Patch)
	patches2, err2 := cr.dmp.PatchFromText(change2.Patch)

	if err1 != nil || err2 != nil {
		return nil // Skip invalid patches
	}

	// Check for overlapping positions
	overlap := cr.checkOverlap(patches1, patches2)
	if overlap {
		return &Conflict{
			Type:        OverlappingEdit,
			Description: "Edits overlap at the same position",
			Agent1:      change1.AgentID,
			Agent2:      change2.AgentID,
			Position:    cr.getOverlapPosition(patches1, patches2),
		}
	}

	// Check for simultaneous edits (same timestamp)
	if change1.Timestamp.Equal(change2.Timestamp) {
		return &Conflict{
			Type:        SimultaneousEdit,
			Description: "Simultaneous edits detected",
			Agent1:      change1.AgentID,
			Agent2:      change2.AgentID,
		}
	}

	return nil
}

// checkOverlap determines if two patches overlap
func (cr *ConflictResolver) checkOverlap(patches1, patches2 []dmp.Patch) bool {
	for _, p1 := range patches1 {
		for _, p2 := range patches2 {
			// Check if patches overlap by comparing start positions and lengths
			start1 := p1.Start1
			end1 := start1 + p1.Length1
			start2 := p2.Start1
			end2 := start2 + p2.Length1

			// Check for overlap
			if (start1 <= start2 && end1 > start2) || (start2 <= start1 && end2 > start1) {
				return true
			}
		}
	}
	return false
}

// getOverlapPosition returns the position where patches overlap
func (cr *ConflictResolver) getOverlapPosition(patches1, patches2 []dmp.Patch) int {
	for _, p1 := range patches1 {
		for _, p2 := range patches2 {
			start1 := p1.Start1
			end1 := start1 + p1.Length1
			start2 := p2.Start1
			end2 := start2 + p2.Length1

			if (start1 <= start2 && end1 > start2) || (start2 <= start1 && end2 > start1) {
				// Return the start of the overlap
				if start1 < start2 {
					return start2
				}
				return start1
			}
		}
	}
	return 0
}

// ResolveConflict attempts to resolve a conflict using various strategies
func (cr *ConflictResolver) ResolveConflict(conflict *Conflict, baseContent string, patches []string) (string, error) {
	switch conflict.Type {
	case OverlappingEdit:
		return cr.resolveOverlappingEdit(baseContent, patches)
	case SimultaneousEdit:
		return cr.resolveSimultaneousEdit(baseContent, patches)
	case ContentConflict:
		return cr.resolveContentConflict(baseContent, patches)
	default:
		return baseContent, fmt.Errorf("unknown conflict type")
	}
}

// resolveOverlappingEdit resolves overlapping edits by merging them
func (cr *ConflictResolver) resolveOverlappingEdit(baseContent string, patches []string) (string, error) {
	// Apply patches in order, handling overlaps
	result := baseContent

	for _, patchStr := range patches {
		patches, err := cr.dmp.PatchFromText(patchStr)
		if err != nil {
			continue
		}

		newResult, results := cr.dmp.PatchApply(patches, result)
		if results[0] {
			result = newResult
		}
		// If patch application fails, we could implement more sophisticated merging
	}

	return result, nil
}

// resolveSimultaneousEdit resolves simultaneous edits by choosing the first one
func (cr *ConflictResolver) resolveSimultaneousEdit(baseContent string, patches []string) (string, error) {
	// For simultaneous edits, apply the first patch
	if len(patches) == 0 {
		return baseContent, nil
	}

	patchList, err := cr.dmp.PatchFromText(patches[0])
	if err != nil {
		return baseContent, err
	}

	newContent, results := cr.dmp.PatchApply(patchList, baseContent)
	if !results[0] {
		return baseContent, fmt.Errorf("failed to apply patch")
	}

	return newContent, nil
}

// resolveContentConflict resolves content conflicts by merging content
func (cr *ConflictResolver) resolveContentConflict(baseContent string, patches []string) (string, error) {
	// For content conflicts, try to merge the changes
	// This is a simplified approach - in practice, you'd want more sophisticated merging

	var mergedContent strings.Builder
	mergedContent.WriteString(baseContent)

	// Apply all patches sequentially
	for _, patchStr := range patches {
		patches, err := cr.dmp.PatchFromText(patchStr)
		if err != nil {
			continue
		}

		newContent, results := cr.dmp.PatchApply(patches, mergedContent.String())
		if results[0] {
			mergedContent.Reset()
			mergedContent.WriteString(newContent)
		}
	}

	return mergedContent.String(), nil
}

// GenerateConflictMarker creates a conflict marker for manual resolution
func (cr *ConflictResolver) GenerateConflictMarker(conflict *Conflict, content1, content2 string) string {
	return fmt.Sprintf(`
<<<<<<< %s
%s
=======
%s
>>>>>>> %s
`, conflict.Agent1, content1, content2, conflict.Agent2)
}

// AutoResolveConflicts attempts to automatically resolve conflicts
func (cr *ConflictResolver) AutoResolveConflicts(conflicts []*Conflict, baseContent string, changes []*ChangeEvent) (string, error) {
	if len(conflicts) == 0 {
		return baseContent, nil
	}

	// Group conflicts by type
	overlappingConflicts := cr.filterConflictsByType(conflicts, OverlappingEdit)
	simultaneousConflicts := cr.filterConflictsByType(conflicts, SimultaneousEdit)
	contentConflicts := cr.filterConflictsByType(conflicts, ContentConflict)

	// Resolve in order of priority
	result := baseContent

	// First resolve overlapping edits
	for _, conflict := range overlappingConflicts {
		patches := cr.getPatchesForConflict(conflict, changes)
		resolved, err := cr.resolveOverlappingEdit(result, patches)
		if err == nil {
			result = resolved
		}
	}

	// Then resolve simultaneous edits
	for _, conflict := range simultaneousConflicts {
		patches := cr.getPatchesForConflict(conflict, changes)
		resolved, err := cr.resolveSimultaneousEdit(result, patches)
		if err == nil {
			result = resolved
		}
	}

	// Finally resolve content conflicts
	for _, conflict := range contentConflicts {
		patches := cr.getPatchesForConflict(conflict, changes)
		resolved, err := cr.resolveContentConflict(result, patches)
		if err == nil {
			result = resolved
		}
	}

	return result, nil
}

// filterConflictsByType filters conflicts by type
func (cr *ConflictResolver) filterConflictsByType(conflicts []*Conflict, conflictType ConflictType) []*Conflict {
	var filtered []*Conflict
	for _, conflict := range conflicts {
		if conflict.Type == conflictType {
			filtered = append(filtered, conflict)
		}
	}
	return filtered
}

// getPatchesForConflict gets patches related to a specific conflict
func (cr *ConflictResolver) getPatchesForConflict(conflict *Conflict, changes []*ChangeEvent) []string {
	var patches []string
	for _, change := range changes {
		if change.AgentID == conflict.Agent1 || change.AgentID == conflict.Agent2 {
			patches = append(patches, change.Patch)
		}
	}
	return patches
}
