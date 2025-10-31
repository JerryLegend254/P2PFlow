package main

import (
	"fmt"
	"os"
	"time"

	"github.com/JerryLegend254/p2pflow/internal/collab"
	"github.com/JerryLegend254/p2pflow/internal/logger"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

func main() {
	// Setup logger
	jsonLogger := logger.NewLogger(logger.JSON)
	jsonLogger.Sync()

	fmt.Println("Testing P2P Collaboration Engine")
	fmt.Println("=====================================")

	// Clean up any existing test files
	cleanupTestFiles()

	// Test 1: Single Agent Patch Generation
	fmt.Println("\nTest 1: Single Agent Patch Generation")
	testSingleAgent(jsonLogger)

	// Test 2: Multiple Agents Concurrent Edits
	fmt.Println("\nTest 2: Multiple Agents Concurrent Edits")
	testMultipleAgents(jsonLogger)

	// Test 3: Conflict Resolution
	fmt.Println("\n Test 3: Conflict Resolution")
	testConflictResolution(jsonLogger)

	// Test 4: Session Persistence
	fmt.Println("\nTest 4: Session Persistence")
	testSessionPersistence(jsonLogger)

	fmt.Println("\n✅ All tests completed!")
}

func cleanupTestFiles() {
	// Remove test files
	os.Remove("test-collab.txt")
	os.RemoveAll(".collab")
}

func testSingleAgent(logger *logger.Logger) {
	// Create a test file
	testFile := "test-collab.txt"
	initialContent := "Hello, this is a test file for collaboration.\nInitial content.\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		logger.Fatalf("Failed to create test file: %v", err)
	}

	// Initialize collaboration engine
	engine := collab.NewCollaborationEngine()
	sessionManager := collab.NewSessionManager(".")

	// Create session
	sessionID := "test-session-1"
	session := engine.CreateSession(sessionID, testFile, initialContent)
	sessionManager.SaveSession(session)

	// Join session as agent
	agentID := "agent-1"
	agentName := "Test Agent 1"
	_, err = engine.JoinSession(sessionID, agentID, agentName)
	if err != nil {
		logger.Fatalf("Failed to join session: %v", err)
	}

	// Simulate file changes
	changes := []string{
		"Hello, this is a test file for collaboration.\nInitial content.\nAdded line 1.\n",
		"Hello, this is a test file for collaboration.\nInitial content.\nAdded line 1.\nAdded line 2.\n",
		"Hello, this is a test file for collaboration.\nInitial content.\nAdded line 1.\nAdded line 2.\nFinal line.\n",
	}

	for i, newContent := range changes {
		// Create change event
		changeEvent := &collab.ChangeEvent{
			SessionID: sessionID,
			AgentID:   agentID,
			Timestamp: time.Now(),
			Patch:     generatePatch(initialContent, newContent),
			Version:   0,
		}

		// Apply change
		updatedSession, err := engine.ApplyChange(changeEvent)
		if err != nil {
			logger.Errorf("Failed to apply change %d: %v", i+1, err)
			continue
		}

		// Save change event
		sessionManager.SaveChangeEvent(changeEvent)
		sessionManager.SaveSession(updatedSession)

		fmt.Printf("  ✅ Change %d applied successfully (Version: %d)\n", i+1, updatedSession.Version)
		initialContent = newContent
	}
}

func testMultipleAgents(logger *logger.Logger) {
	// Create a test file
	testFile := "test-collab.txt"
	initialContent := "Collaborative editing test.\nBase content.\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		logger.Fatalf("Failed to create test file: %v", err)
	}

	// Initialize collaboration engine
	engine := collab.NewCollaborationEngine()
	sessionManager := collab.NewSessionManager(".")

	// Create session
	sessionID := "test-session-2"
	session := engine.CreateSession(sessionID, testFile, initialContent)
	sessionManager.SaveSession(session)

	// Join multiple agents
	agents := []struct {
		ID   string
		Name string
	}{
		{"agent-1", "Alice"},
		{"agent-2", "Bob"},
		{"agent-3", "Charlie"},
	}

	for _, agent := range agents {
		_, err = engine.JoinSession(sessionID, agent.ID, agent.Name)
		if err != nil {
			logger.Fatalf("Failed to join session as %s: %v", agent.Name, err)
		}
		fmt.Printf("  ✅ %s joined the session\n", agent.Name)
	}

	// Simulate concurrent edits
	edits := []struct {
		AgentID string
		Content string
	}{
		{"agent-1", "Collaborative editing test.\nBase content.\nAlice's addition.\n"},
		{"agent-2", "Collaborative editing test.\nBase content.\nBob's contribution.\n"},
		{"agent-3", "Collaborative editing test.\nBase content.\nCharlie's input.\n"},
	}

	for _, edit := range edits {
		changeEvent := &collab.ChangeEvent{
			SessionID: sessionID,
			AgentID:   edit.AgentID,
			Timestamp: time.Now(),
			Patch:     generatePatch(initialContent, edit.Content),
			Version:   0,
		}

		// Apply change
		updatedSession, err := engine.ApplyChange(changeEvent)
		if err != nil {
			logger.Errorf("Failed to apply change from %s: %v", edit.AgentID, err)
			continue
		}

		// Save change event
		sessionManager.SaveChangeEvent(changeEvent)
		sessionManager.SaveSession(updatedSession)

		fmt.Printf("  ✅ %s's edit applied (Version: %d)\n", edit.AgentID, updatedSession.Version)
	}
}

func testConflictResolution(logger *logger.Logger) {
	// Create a test file
	testFile := "test-collab.txt"
	initialContent := "Conflict resolution test.\nOriginal content.\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		logger.Fatalf("Failed to create test file: %v", err)
	}

	// Initialize collaboration engine and conflict resolver
	engine := collab.NewCollaborationEngine()
	sessionManager := collab.NewSessionManager(".")
	conflictResolver := collab.NewConflictResolver()

	// Create session
	sessionID := "test-session-3"
	session := engine.CreateSession(sessionID, testFile, initialContent)
	sessionManager.SaveSession(session)

	// Join agents
	_, err = engine.JoinSession(sessionID, "agent-1", "Alice")
	if err != nil {
		logger.Fatalf("Failed to join session: %v", err)
	}
	_, err = engine.JoinSession(sessionID, "agent-2", "Bob")
	if err != nil {
		logger.Fatalf("Failed to join session: %v", err)
	}

	// Create conflicting changes (same position)
	conflictTime := time.Now()
	change1 := &collab.ChangeEvent{
		SessionID: sessionID,
		AgentID:   "agent-1",
		Timestamp: conflictTime,
		Patch:     generatePatch(initialContent, "Conflict resolution test.\nAlice's version.\n"),
		Version:   0,
	}

	change2 := &collab.ChangeEvent{
		SessionID: sessionID,
		AgentID:   "agent-2",
		Timestamp: conflictTime, // Same timestamp = simultaneous edit
		Patch:     generatePatch(initialContent, "Conflict resolution test.\nBob's version.\n"),
		Version:   0,
	}

	// Detect conflicts
	conflicts := conflictResolver.DetectConflicts([]*collab.ChangeEvent{change1, change2})
	fmt.Printf("  Detected %d conflicts\n", len(conflicts))

	for i, conflict := range conflicts {
		fmt.Printf("  Conflict %d: %s (Type: %d)\n", i+1, conflict.Description, conflict.Type)
	}

	// Resolve conflicts
	if len(conflicts) > 0 {
		resolvedContent, err := conflictResolver.AutoResolveConflicts(conflicts, initialContent, []*collab.ChangeEvent{change1, change2})
		if err != nil {
			logger.Errorf("Failed to resolve conflicts: %v", err)
		} else {
			fmt.Printf("  ✅ Conflicts resolved automatically\n")
			fmt.Printf("   Resolved content: %s\n", resolvedContent)
		}
	}
}

func testSessionPersistence(logger *logger.Logger) {
	// Create a test file
	testFile := "test-collab.txt"
	initialContent := "Session persistence test.\nTesting metadata storage.\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		logger.Fatalf("Failed to create test file: %v", err)
	}

	// Initialize collaboration engine
	engine := collab.NewCollaborationEngine()
	sessionManager := collab.NewSessionManager(".")

	// Create session
	sessionID := "test-session-4"
	session := engine.CreateSession(sessionID, testFile, initialContent)
	sessionManager.SaveSession(session)

	// Join agent
	_, err = engine.JoinSession(sessionID, "agent-1", "Test Agent")
	if err != nil {
		logger.Fatalf("Failed to join session: %v", err)
	}

	// Make some changes
	changeEvent := &collab.ChangeEvent{
		SessionID: sessionID,
		AgentID:   "agent-1",
		Timestamp: time.Now(),
		Patch:     generatePatch(initialContent, "Session persistence test.\nTesting metadata storage.\nAdded persistence test.\n"),
		Version:   0,
	}

	updatedSession, err := engine.ApplyChange(changeEvent)
	if err != nil {
		logger.Fatalf("Failed to apply change: %v", err)
	}

	sessionManager.SaveChangeEvent(changeEvent)
	sessionManager.SaveSession(updatedSession)

	// Verify persistence
	savedSessions, err := sessionManager.ListSessions()
	if err != nil {
		logger.Fatalf("Failed to list sessions: %v", err)
	}

	fmt.Printf("  Found %d saved sessions\n", len(savedSessions))
	for _, savedSession := range savedSessions {
		fmt.Printf("  Session: %s (Version: %d, Agents: %d)\n",
			savedSession.ID, savedSession.Version, len(savedSession.Agents))
	}

	// Load change events
	events, err := sessionManager.LoadChangeEvents(sessionID)
	if err != nil {
		logger.Fatalf("Failed to load change events: %v", err)
	}

	fmt.Printf("  Found %d change events\n", len(events))
	for i, event := range events {
		fmt.Printf("  Event %d: Agent %s at %s\n", i+1, event.AgentID, event.Timestamp.Format(time.RFC3339))
	}
}

// Helper function to generate patches
func generatePatch(oldContent, newContent string) string {
	dmp := dmp.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)
	patches := dmp.PatchMake(diffs)
	return dmp.PatchToText(patches)
}
