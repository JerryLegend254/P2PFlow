package crdt

import (
	"testing"
)

func TestCRDTEngine_CreateSession(t *testing.T) {
	engine := NewCRDTEngine()

	session := engine.CreateSession("session1", "/tmp/test", "agent1")

	if session.ID != "session1" {
		t.Errorf("Expected session ID 'session1', got %s", session.ID)
	}

	if session.RootPath != "/tmp/test" {
		t.Errorf("Expected root path '/tmp/test', got %s", session.RootPath)
	}

	if len(session.Documents) != 0 {
		t.Errorf("Expected 0 documents initially, got %d", len(session.Documents))
	}
}

func TestCRDTEngine_JoinSession(t *testing.T) {
	engine := NewCRDTEngine()

	// Create session
	engine.CreateSession("session1", "/tmp/test", "agent1")

	// Join session
	session, err := engine.JoinSession("session1", "agent2", "Agent 2")
	if err != nil {
		t.Fatalf("Failed to join session: %v", err)
	}

	if len(session.Agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(session.Agents))
	}

	agent := session.Agents["agent2"]
	if agent == nil {
		t.Fatal("Agent not found in session")
	}

	if agent.Name != "Agent 2" {
		t.Errorf("Expected agent name 'Agent 2', got %s", agent.Name)
	}
}

func TestCRDTEngine_GetOrCreateDocument(t *testing.T) {
	engine := NewCRDTEngine()

	// Create session
	engine.CreateSession("session1", "/tmp/test", "agent1")

	// Get or create document
	doc, err := engine.GetOrCreateDocument("session1", "test.txt", "agent1")
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}

	if doc == nil {
		t.Fatal("Expected document, got nil")
	}

	// Get same document again
	doc2, err := engine.GetOrCreateDocument("session1", "test.txt", "agent1")
	if err != nil {
		t.Fatalf("Failed to get document second time: %v", err)
	}

	// Should be the same document
	if doc != doc2 {
		t.Error("Expected same document instance")
	}
}

func TestCRDTEngine_InitializeDocument(t *testing.T) {
	engine := NewCRDTEngine()

	// Create session
	engine.CreateSession("session1", "/tmp/test", "agent1")

	// Initialize document with content
	content := "Line 1\nLine 2\nLine 3"
	err := engine.InitializeDocument("session1", "test.txt", "agent1", content)
	if err != nil {
		t.Fatalf("Failed to initialize document: %v", err)
	}

	// Get content
	docContent, err := engine.GetDocumentContent("session1", "test.txt")
	if err != nil {
		t.Fatalf("Failed to get document content: %v", err)
	}

	if docContent != content {
		t.Errorf("Expected content '%s', got '%s'", content, docContent)
	}
}

func TestCRDTEngine_ApplyOperation(t *testing.T) {
	engine := NewCRDTEngine()

	// Create session and initialize document
	engine.CreateSession("session1", "/tmp/test", "agent1")
	engine.InitializeDocument("session1", "test.txt", "agent1", "Line 1")

	// Get document
	doc, _ := engine.GetOrCreateDocument("session1", "test.txt", "agent1")

	// Create an operation
	op, err := doc.Insert(1, "Line 2")
	if err != nil {
		t.Fatalf("Failed to create operation: %v", err)
	}

	// Apply operation
	err = engine.ApplyOperation("session1", "test.txt", op)
	if err != nil {
		t.Fatalf("Failed to apply operation: %v", err)
	}

	// Check content
	content, _ := engine.GetDocumentContent("session1", "test.txt")
	expected := "Line 1\nLine 2"
	if content != expected {
		t.Errorf("Expected content '%s', got '%s'", expected, content)
	}
}

func TestCRDTEngine_SyncOperations(t *testing.T) {
	engine := NewCRDTEngine()

	// Create session and document
	session := engine.CreateSession("session1", "/tmp/test", "agent1")
	engine.InitializeDocument("session1", "test.txt", "agent1", "Line 1")

	// Get document and create some operations
	doc, _ := engine.GetOrCreateDocument("session1", "test.txt", "agent1")
	op1, _ := doc.Insert(1, "Line 2")
	op2, _ := doc.Insert(2, "Line 3")

	// Apply operations
	engine.ApplyOperation("session1", "test.txt", op1)
	engine.ApplyOperation("session1", "test.txt", op2)

	// Create a vector clock for another node that hasn't seen any operations
	theirClock := NewVectorClock()

	// Sync operations
	missingOps, err := engine.SyncOperations("session1", theirClock)
	if err != nil {
		t.Fatalf("Failed to sync operations: %v", err)
	}

	// Should return both operations
	if len(missingOps) < 2 {
		t.Errorf("Expected at least 2 missing operations, got %d", len(missingOps))
	}

	// Create a clock that has seen op1
	theirClock2 := session.VectorClock.Copy()

	// Sync again - should return no operations
	missingOps2, err := engine.SyncOperations("session1", theirClock2)
	if err != nil {
		t.Fatalf("Failed to sync operations: %v", err)
	}

	if len(missingOps2) != 0 {
		t.Errorf("Expected 0 missing operations, got %d", len(missingOps2))
	}
}

func TestCRDTEngine_GetStats(t *testing.T) {
	engine := NewCRDTEngine()

	// Create session
	engine.CreateSession("session1", "/tmp/test", "agent1")
	engine.JoinSession("session1", "agent2", "Agent 2")

	// Initialize two documents
	engine.InitializeDocument("session1", "test1.txt", "agent1", "Content 1")
	engine.InitializeDocument("session1", "test2.txt", "agent1", "Content 2")

	// Get document and create operation
	doc, _ := engine.GetOrCreateDocument("session1", "test1.txt", "agent1")
	op, _ := doc.Insert(1, "New line")
	engine.ApplyOperation("session1", "test1.txt", op)

	// Get stats
	stats, err := engine.GetStats("session1")
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.SessionID != "session1" {
		t.Errorf("Expected session ID 'session1', got %s", stats.SessionID)
	}

	if stats.DocumentCount != 2 {
		t.Errorf("Expected 2 documents, got %d", stats.DocumentCount)
	}

	if stats.AgentCount != 1 {
		t.Errorf("Expected 1 agent, got %d", stats.AgentCount)
	}

	if stats.OperationCount < 1 {
		t.Errorf("Expected at least 1 operation, got %d", stats.OperationCount)
	}
}

func TestCRDTEngine_ListDocuments(t *testing.T) {
	engine := NewCRDTEngine()

	// Create session
	engine.CreateSession("session1", "/tmp/test", "agent1")

	// Initialize documents
	engine.InitializeDocument("session1", "test1.txt", "agent1", "Content 1")
	engine.InitializeDocument("session1", "test2.txt", "agent1", "Content 2")
	engine.InitializeDocument("session1", "test3.txt", "agent1", "Content 3")

	// List documents
	docs, err := engine.ListDocuments("session1")
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}

	if len(docs) != 3 {
		t.Errorf("Expected 3 documents, got %d", len(docs))
	}

	// Check that all expected documents are present
	expectedDocs := map[string]bool{
		"test1.txt": false,
		"test2.txt": false,
		"test3.txt": false,
	}

	for _, doc := range docs {
		if _, exists := expectedDocs[doc]; exists {
			expectedDocs[doc] = true
		}
	}

	for doc, found := range expectedDocs {
		if !found {
			t.Errorf("Document %s not found in list", doc)
		}
	}
}

func TestCRDTEngine_MultipleAgents(t *testing.T) {
	engine := NewCRDTEngine()

	// Agent 1 creates session
	engine.CreateSession("session1", "/tmp/test", "agent1")
	engine.InitializeDocument("session1", "test.txt", "agent1", "Initial content")

	// Agent 2 joins
	engine.JoinSession("session1", "agent2", "Agent 2")

	// Both agents get the document
	doc1, _ := engine.GetOrCreateDocument("session1", "test.txt", "agent1")
	doc2, _ := engine.GetOrCreateDocument("session1", "test.txt", "agent2")

	// Agent 1 inserts a line
	op1, _ := doc1.Insert(1, "From Agent 1")
	engine.ApplyOperation("session1", "test.txt", op1)

	// Agent 2 inserts a line
	op2, _ := doc2.Insert(1, "From Agent 2")
	engine.ApplyOperation("session1", "test.txt", op2)

	// Both operations should be in the operation log
	session, _ := engine.GetSession("session1")
	if len(session.OperationLog) < 2 {
		t.Errorf("Expected at least 2 operations in log, got %d", len(session.OperationLog))
	}

	// Final content should be deterministic
	content, _ := engine.GetDocumentContent("session1", "test.txt")
	if content == "" {
		t.Error("Expected non-empty content")
	}
}
