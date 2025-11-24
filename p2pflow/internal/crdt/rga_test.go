package crdt

import (
	"testing"
)

func TestRGADocument_InsertAndGetContent(t *testing.T) {
	doc := NewRGADocument("node1")

	// Insert first line
	op1, err := doc.Insert(0, "Hello World")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}
	if op1 == nil {
		t.Fatal("Expected operation, got nil")
	}

	content := doc.GetContent()
	if len(content) != 1 || content[0] != "Hello World" {
		t.Errorf("Expected ['Hello World'], got %v", content)
	}

	// Insert second line
	op2, err := doc.Insert(1, "Second line")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}
	if op2 == nil {
		t.Fatal("Expected operation, got nil")
	}

	content = doc.GetContent()
	if len(content) != 2 || content[1] != "Second line" {
		t.Errorf("Expected 2 lines, got %v", content)
	}
}

func TestRGADocument_Delete(t *testing.T) {
	doc := NewRGADocument("node1")

	// Insert three lines
	doc.Insert(0, "Line 1")
	doc.Insert(1, "Line 2")
	doc.Insert(2, "Line 3")

	// Delete middle line
	op, err := doc.Delete(1)
	if err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}
	if op == nil {
		t.Fatal("Expected operation, got nil")
	}

	content := doc.GetContent()
	if len(content) != 2 {
		t.Errorf("Expected 2 lines after delete, got %d", len(content))
	}
	if content[0] != "Line 1" || content[1] != "Line 3" {
		t.Errorf("Expected ['Line 1', 'Line 3'], got %v", content)
	}
}

func TestRGADocument_ConcurrentInserts(t *testing.T) {
	// Simulate two nodes inserting at the same position with shared initial state
	doc1 := NewRGADocument("node1")

	// Node1 creates initial content
	initOp, _ := doc1.Insert(0, "Line 1")

	// Node2 replicates the initial state
	doc2 := NewRGADocument("node2")
	doc2.ApplyOperation(initOp)

	// Verify both have same initial state
	if doc1.GetContentAsString() != doc2.GetContentAsString() {
		t.Fatal("Initial states don't match")
	}

	// Node1 inserts at position 1
	op1, _ := doc1.Insert(1, "From Node1")

	// Node2 inserts at position 1 concurrently
	op2, _ := doc2.Insert(1, "From Node2")

	// Exchange operations
	doc1.ApplyOperation(op2)
	doc2.ApplyOperation(op1)

	// Both documents should converge to the same state
	content1 := doc1.GetContent()
	content2 := doc2.GetContent()

	if len(content1) != len(content2) {
		t.Errorf("Documents diverged in length: doc1=%d, doc2=%d", len(content1), len(content2))
		t.Errorf("doc1=%v, doc2=%v", content1, content2)
		return
	}

	// Check that both have 3 lines total
	if len(content1) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(content1))
	}

	// Both should converge to same content in same order
	// RGA uses timestamp-based ordering, so the result is deterministic
	for i := range content1 {
		if content1[i] != content2[i] {
			t.Errorf("Line %d differs: doc1=%s, doc2=%s", i, content1[i], content2[i])
		}
	}

	// Verify that both lines from nodes are present
	contentStr := doc1.GetContentAsString()
	if !contains(contentStr, "From Node1") {
		t.Error("Missing 'From Node1' in final content")
	}
	if !contains(contentStr, "From Node2") {
		t.Error("Missing 'From Node2' in final content")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func TestRGADocument_OutOfOrderOperations(t *testing.T) {
	doc1 := NewRGADocument("node1")
	doc2 := NewRGADocument("node2")

	// Node1 creates operations
	op1, _ := doc1.Insert(0, "Line 1")
	op2, _ := doc1.Insert(1, "Line 2")
	op3, _ := doc1.Insert(2, "Line 3")

	// Node2 receives operations out of order: 3, 1, 2
	doc2.ApplyOperation(op3)
	doc2.ApplyOperation(op1)
	doc2.ApplyOperation(op2)

	// Both documents should have the same content
	content1 := doc1.GetContent()
	content2 := doc2.GetContent()

	if len(content1) != len(content2) {
		t.Errorf("Documents diverged after out-of-order ops: doc1=%v, doc2=%v", content1, content2)
	}
}

func TestRGADocument_SetContent(t *testing.T) {
	doc := NewRGADocument("node1")

	lines := []string{"Line 1", "Line 2", "Line 3"}
	err := doc.SetContent(lines)
	if err != nil {
		t.Fatalf("Failed to set content: %v", err)
	}

	content := doc.GetContent()
	if len(content) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(content))
	}

	for i, line := range lines {
		if content[i] != line {
			t.Errorf("Line %d: expected %s, got %s", i, line, content[i])
		}
	}
}

func TestRGADocument_Idempotency(t *testing.T) {
	doc := NewRGADocument("node1")

	// Insert a line
	op, _ := doc.Insert(0, "Test line")

	// Apply the same operation twice
	doc.ApplyOperation(op)

	content := doc.GetContent()
	if len(content) != 1 {
		t.Errorf("Expected 1 line (idempotency), got %d", len(content))
	}
}

func TestRGADocument_Stats(t *testing.T) {
	doc := NewRGADocument("node1")

	// Insert and delete some lines
	doc.Insert(0, "Line 1")
	doc.Insert(1, "Line 2")
	doc.Insert(2, "Line 3")
	doc.Delete(1) // Delete "Line 2"

	stats := doc.GetStats()

	if stats.TotalElements != 3 {
		t.Errorf("Expected 3 total elements, got %d", stats.TotalElements)
	}

	if stats.VisibleElements != 2 {
		t.Errorf("Expected 2 visible elements, got %d", stats.VisibleElements)
	}

	if stats.Tombstones != 1 {
		t.Errorf("Expected 1 tombstone, got %d", stats.Tombstones)
	}
}

func TestElementID_Compare(t *testing.T) {
	// Later timestamp should be less than (sort before) earlier timestamp
	id1 := ElementID{Timestamp: 100, ReplicaID: "node1"}
	id2 := ElementID{Timestamp: 50, ReplicaID: "node2"}

	if id1.Compare(id2) != -1 {
		t.Error("Expected id1 < id2 (later timestamp wins)")
	}

	if id2.Compare(id1) != 1 {
		t.Error("Expected id2 > id1")
	}

	// Same timestamp, use replica ID
	id3 := ElementID{Timestamp: 100, ReplicaID: "node1"}
	id4 := ElementID{Timestamp: 100, ReplicaID: "node2"}

	result := id3.Compare(id4)
	if result != -1 {
		t.Errorf("Expected id3 < id4 (replica ID ordering), got %d", result)
	}
}

func TestOperationBuffer(t *testing.T) {
	buffer := NewOperationBuffer()

	op1 := &Operation{Timestamp: 100}
	op2 := &Operation{Timestamp: 50}
	op3 := &Operation{Timestamp: 75}

	buffer.Add(op1)
	buffer.Add(op2)
	buffer.Add(op3)

	if buffer.Len() != 3 {
		t.Errorf("Expected 3 operations, got %d", buffer.Len())
	}

	ordered := buffer.GetOrdered()
	if len(ordered) != 3 {
		t.Errorf("Expected 3 ordered operations, got %d", len(ordered))
	}

	// Should be sorted by timestamp
	if ordered[0].Timestamp != 50 || ordered[1].Timestamp != 75 || ordered[2].Timestamp != 100 {
		t.Error("Operations not properly ordered by timestamp")
	}

	buffer.Clear()
	if buffer.Len() != 0 {
		t.Errorf("Expected 0 operations after clear, got %d", buffer.Len())
	}
}

func TestRGADocument_ComplexScenario(t *testing.T) {
	// Simulate a realistic collaborative editing scenario
	doc1 := NewRGADocument("alice")
	doc2 := NewRGADocument("bob")

	// Alice starts a document and creates initial operations
	op1, _ := doc1.Insert(0, "# README")
	op2, _ := doc1.Insert(1, "")
	op3, _ := doc1.Insert(2, "This is a test")

	// Bob replicates the initial state
	doc2.ApplyOperation(op1)
	doc2.ApplyOperation(op2)
	doc2.ApplyOperation(op3)

	// Verify both have same initial state
	if doc1.GetContentAsString() != doc2.GetContentAsString() {
		t.Fatal("Initial states don't match")
	}

	// Alice adds a line
	opAlice1, _ := doc1.Insert(3, "Added by Alice")

	// Bob adds a line at the same time
	opBob1, _ := doc2.Insert(3, "Added by Bob")

	// Alice deletes a line (line 2 = "This is a test")
	opAlice2, _ := doc1.Delete(2)

	// Exchange operations
	doc1.ApplyOperation(opBob1)
	doc2.ApplyOperation(opAlice1)
	doc2.ApplyOperation(opAlice2)
	doc1.ApplyOperation(opAlice2) // Alice's own delete (idempotency test)

	// Both should converge
	content1 := doc1.GetContent()
	content2 := doc2.GetContent()

	if len(content1) != len(content2) {
		t.Errorf("Documents diverged in length: alice=%d, bob=%d", len(content1), len(content2))
		t.Errorf("alice=%v, bob=%v", content1, content2)
		return
	}

	for i := range content1 {
		if content1[i] != content2[i] {
			t.Errorf("Line %d differs: alice=%s, bob=%s", i, content1[i], content2[i])
		}
	}

	// Should have "# README", "", and the two new lines (4 total after delete)
	if len(content1) != 4 {
		t.Errorf("Expected 4 lines, got %d: %v", len(content1), content1)
	}

	// Verify expected lines are present
	contentStr := doc1.GetContentAsString()
	if !contains(contentStr, "# README") {
		t.Error("Missing '# README' in final content")
	}
	if !contains(contentStr, "Added by Alice") {
		t.Error("Missing 'Added by Alice' in final content")
	}
	if !contains(contentStr, "Added by Bob") {
		t.Error("Missing 'Added by Bob' in final content")
	}
	if contains(contentStr, "This is a test") {
		t.Error("'This is a test' should have been deleted")
	}
}
