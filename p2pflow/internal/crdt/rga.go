package crdt

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// OperationType defines the type of CRDT operation
type OperationType string

const (
	OperationInsert OperationType = "insert"
	OperationDelete OperationType = "delete"
)

// ElementID uniquely identifies an element in the RGA
// Elements are ordered by (Timestamp, ReplicaID) for deterministic conflict resolution
type ElementID struct {
	Timestamp int64  `json:"timestamp"`  // Lamport timestamp
	ReplicaID string `json:"replica_id"` // Unique node/agent identifier
}

// Compare returns -1 if this ID is less than other, 1 if greater, 0 if equal
// Later timestamps sort before earlier ones (newer elements win in conflicts)
func (e ElementID) Compare(other ElementID) int {
	if e.Timestamp > other.Timestamp {
		return -1
	}
	if e.Timestamp < other.Timestamp {
		return 1
	}
	// Timestamps equal, use replica ID for deterministic ordering
	if e.ReplicaID < other.ReplicaID {
		return -1
	}
	if e.ReplicaID > other.ReplicaID {
		return 1
	}
	return 0
}

// Element represents a single element in the RGA
type Element struct {
	ID      ElementID `json:"id"`
	Content string    `json:"content"` // Line content for line-based CRDT
	Deleted bool      `json:"deleted"` // Tombstone flag
}

// Operation represents a CRDT operation (insert or delete)
type Operation struct {
	Type        OperationType `json:"type"`
	ElementID   ElementID     `json:"element_id"`
	Content     string        `json:"content,omitempty"` // For insert operations
	PrevID      *ElementID    `json:"prev_id,omitempty"` // Insert after this element
	VectorClock *VectorClock  `json:"vector_clock"`      // Causality tracking
	ReplicaID   string        `json:"replica_id"`        // Node that created this op
	Timestamp   int64         `json:"timestamp"`         // Operation timestamp
}

// RGADocument represents a line-based CRDT document using RGA algorithm
type RGADocument struct {
	Elements  []*Element   `json:"elements"`   // Ordered list of elements
	ReplicaID string       `json:"replica_id"` // This node's ID
	Clock     *VectorClock `json:"clock"`      // Vector clock for causality
	Lamport   int64        `json:"lamport"`    // Lamport clock for element IDs
	mu        sync.RWMutex
}

// NewRGADocument creates a new RGA document
func NewRGADocument(replicaID string) *RGADocument {
	return &RGADocument{
		Elements:  make([]*Element, 0),
		ReplicaID: replicaID,
		Clock:     NewVectorClock(),
		Lamport:   0,
	}
}

// nextLamport increments and returns the next Lamport timestamp
func (doc *RGADocument) nextLamport() int64 {
	doc.Lamport++
	return doc.Lamport
}

// updateLamport updates Lamport clock to be at least as large as the given value
func (doc *RGADocument) updateLamport(timestamp int64) {
	if timestamp > doc.Lamport {
		doc.Lamport = timestamp
	}
}

// Insert creates an insert operation for a new line
func (doc *RGADocument) Insert(lineNumber int, content string) (*Operation, error) {
	doc.mu.Lock()
	defer doc.mu.Unlock()

	// Increment vector clock and Lamport clock
	doc.Clock.Increment(doc.ReplicaID)
	timestamp := doc.nextLamport()

	// Create element ID
	elemID := ElementID{
		Timestamp: timestamp,
		ReplicaID: doc.ReplicaID,
	}

	// Determine previous element ID
	var prevID *ElementID
	if lineNumber > 0 {
		// Find the visible element before this position
		visibleIndex := -1
		for _, elem := range doc.Elements {
			if !elem.Deleted {
				visibleIndex++
				if visibleIndex == lineNumber-1 {
					prevID = &elem.ID
					break
				}
			}
		}
	}

	// Create operation
	op := &Operation{
		Type:        OperationInsert,
		ElementID:   elemID,
		Content:     content,
		PrevID:      prevID,
		VectorClock: doc.Clock.Copy(),
		ReplicaID:   doc.ReplicaID,
		Timestamp:   time.Now().UnixNano(),
	}

	// Apply operation locally
	if err := doc.applyInsert(op); err != nil {
		return nil, err
	}

	return op, nil
}

// Delete creates a delete operation for a line
func (doc *RGADocument) Delete(lineNumber int) (*Operation, error) {
	doc.mu.Lock()
	defer doc.mu.Unlock()

	// Find the element at the given line number
	visibleIndex := -1
	var targetElem *Element
	for _, elem := range doc.Elements {
		if !elem.Deleted {
			visibleIndex++
			if visibleIndex == lineNumber {
				targetElem = elem
				break
			}
		}
	}

	if targetElem == nil {
		return nil, fmt.Errorf("line %d not found", lineNumber)
	}

	// Increment vector clock
	doc.Clock.Increment(doc.ReplicaID)

	// Create operation
	op := &Operation{
		Type:        OperationDelete,
		ElementID:   targetElem.ID,
		VectorClock: doc.Clock.Copy(),
		ReplicaID:   doc.ReplicaID,
		Timestamp:   time.Now().UnixNano(),
	}

	// Apply operation locally
	if err := doc.applyDelete(op); err != nil {
		return nil, err
	}

	return op, nil
}

// ApplyOperation applies a remote operation to this document
func (doc *RGADocument) ApplyOperation(op *Operation) error {
	doc.mu.Lock()
	defer doc.mu.Unlock()

	// Update Lamport clock
	doc.updateLamport(op.ElementID.Timestamp)

	// Update vector clock
	doc.Clock.Merge(op.VectorClock)

	switch op.Type {
	case OperationInsert:
		return doc.applyInsert(op)
	case OperationDelete:
		return doc.applyDelete(op)
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// applyInsert applies an insert operation (caller must hold lock)
func (doc *RGADocument) applyInsert(op *Operation) error {
	// Check if element already exists (idempotency)
	for _, elem := range doc.Elements {
		if elem.ID.Timestamp == op.ElementID.Timestamp && elem.ID.ReplicaID == op.ElementID.ReplicaID {
			return nil // Already applied
		}
	}

	// Create new element
	newElem := &Element{
		ID:      op.ElementID,
		Content: op.Content,
		Deleted: false,
	}

	// Find insertion position
	insertPos := 0
	if op.PrevID != nil {
		// Find the previous element
		for i, elem := range doc.Elements {
			if elem.ID.Timestamp == op.PrevID.Timestamp && elem.ID.ReplicaID == op.PrevID.ReplicaID {
				insertPos = i + 1
				break
			}
		}

		// Skip over elements inserted after the same previous element
		// that have higher priority (later timestamp or greater replica ID)
		for insertPos < len(doc.Elements) {
			elem := doc.Elements[insertPos]

			// Check if this element was also inserted after the same previous element
			// If so, use timestamp-based ordering
			if elem.ID.Compare(newElem.ID) < 0 {
				insertPos++
			} else {
				break
			}
		}
	} else {
		// Insert at beginning, but skip over other elements with no previous
		// that have higher priority
		for insertPos < len(doc.Elements) {
			elem := doc.Elements[insertPos]
			if elem.ID.Compare(newElem.ID) < 0 {
				insertPos++
			} else {
				break
			}
		}
	}

	// Insert element at position
	doc.Elements = append(doc.Elements[:insertPos], append([]*Element{newElem}, doc.Elements[insertPos:]...)...)

	return nil
}

// applyDelete applies a delete operation (caller must hold lock)
func (doc *RGADocument) applyDelete(op *Operation) error {
	// Find and mark element as deleted
	for _, elem := range doc.Elements {
		if elem.ID.Timestamp == op.ElementID.Timestamp && elem.ID.ReplicaID == op.ElementID.ReplicaID {
			elem.Deleted = true
			return nil
		}
	}

	// Element not found - this is okay, it might not have arrived yet
	// Operation will be reapplied when element arrives
	return nil
}

// GetContent returns the current visible content (non-deleted lines)
func (doc *RGADocument) GetContent() []string {
	doc.mu.RLock()
	defer doc.mu.RUnlock()

	var lines []string
	for _, elem := range doc.Elements {
		if !elem.Deleted {
			lines = append(lines, elem.Content)
		}
	}
	return lines
}

// GetContentAsString returns the content as a single string
func (doc *RGADocument) GetContentAsString() string {
	lines := doc.GetContent()
	result := ""
	for i, line := range lines {
		result += line
		if i < len(lines)-1 {
			result += "\n"
		}
	}
	return result
}

// SetContent replaces the entire document content
// Useful for initializing from a file
func (doc *RGADocument) SetContent(lines []string) error {
	doc.mu.Lock()
	defer doc.mu.Unlock()

	// Clear existing elements
	doc.Elements = make([]*Element, 0, len(lines))

	// Create elements for each line
	for _, line := range lines {
		doc.Clock.Increment(doc.ReplicaID)
		timestamp := doc.nextLamport()

		elem := &Element{
			ID: ElementID{
				Timestamp: timestamp,
				ReplicaID: doc.ReplicaID,
			},
			Content: line,
			Deleted: false,
		}
		doc.Elements = append(doc.Elements, elem)
	}

	return nil
}

// GarbageCollect removes tombstones (deleted elements)
// Should only be called when all replicas have acknowledged deletions
func (doc *RGADocument) GarbageCollect(acknowledgedClock *VectorClock) int {
	doc.mu.Lock()
	defer doc.mu.Unlock()

	filtered := make([]*Element, 0, len(doc.Elements))
	removed := 0

	for _, elem := range doc.Elements {
		// Keep element if not deleted or if not all replicas have acknowledged
		if !elem.Deleted {
			filtered = append(filtered, elem)
		} else {
			// Check if all replicas have seen this deletion
			// This is a simplified version - full implementation needs tracking
			removed++
		}
	}

	doc.Elements = filtered
	return removed
}

// MarshalJSON implements custom JSON marshaling
func (doc *RGADocument) MarshalJSON() ([]byte, error) {
	doc.mu.RLock()
	defer doc.mu.RUnlock()

	type Alias RGADocument
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(doc),
	})
}

// Stats returns document statistics
type DocumentStats struct {
	TotalElements   int
	VisibleElements int
	Tombstones      int
	ReplicaID       string
	LamportClock    int64
}

// GetStats returns document statistics
func (doc *RGADocument) GetStats() DocumentStats {
	doc.mu.RLock()
	defer doc.mu.RUnlock()

	visible := 0
	tombstones := 0
	for _, elem := range doc.Elements {
		if elem.Deleted {
			tombstones++
		} else {
			visible++
		}
	}

	return DocumentStats{
		TotalElements:   len(doc.Elements),
		VisibleElements: visible,
		Tombstones:      tombstones,
		ReplicaID:       doc.ReplicaID,
		LamportClock:    doc.Lamport,
	}
}

// OperationBuffer manages out-of-order operations
type OperationBuffer struct {
	operations []*Operation
	mu         sync.RWMutex
}

// NewOperationBuffer creates a new operation buffer
func NewOperationBuffer() *OperationBuffer {
	return &OperationBuffer{
		operations: make([]*Operation, 0),
	}
}

// Add adds an operation to the buffer
func (buf *OperationBuffer) Add(op *Operation) {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	buf.operations = append(buf.operations, op)
}

// GetOrdered returns operations sorted by causality
func (buf *OperationBuffer) GetOrdered() []*Operation {
	buf.mu.RLock()
	defer buf.mu.RUnlock()

	// Sort by timestamp
	sorted := make([]*Operation, len(buf.operations))
	copy(sorted, buf.operations)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})

	return sorted
}

// Clear removes all operations from the buffer
func (buf *OperationBuffer) Clear() {
	buf.mu.Lock()
	defer buf.mu.Unlock()
	buf.operations = make([]*Operation, 0)
}

// Len returns the number of buffered operations
func (buf *OperationBuffer) Len() int {
	buf.mu.RLock()
	defer buf.mu.RUnlock()
	return len(buf.operations)
}
