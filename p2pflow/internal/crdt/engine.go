package crdt

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CRDTSession represents a CRDT-based collaboration session
type CRDTSession struct {
	ID           string                  `json:"id"`
	RootPath     string                  `json:"root_path"`
	Documents    map[string]*RGADocument `json:"documents"` // Map of file path -> RGA document
	Agents       map[string]*Agent       `json:"agents"`
	CreatedAt    time.Time               `json:"created_at"`
	VectorClock  *VectorClock            `json:"vector_clock"`
	OperationLog []*Operation            `json:"operation_log"` // For persistence and sync
}

// Agent represents a participant in the CRDT session
type Agent struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	LastSeen time.Time `json:"last_seen"`
}

// CRDTEngine manages CRDT-based collaboration sessions
type CRDTEngine struct {
	sessions map[string]*CRDTSession
	mu       sync.RWMutex
}

// NewCRDTEngine creates a new CRDT collaboration engine
func NewCRDTEngine() *CRDTEngine {
	return &CRDTEngine{
		sessions: make(map[string]*CRDTSession),
	}
}

// CreateSession creates a new CRDT collaboration session
func (ce *CRDTEngine) CreateSession(sessionID, rootPath, agentID string) *CRDTSession {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session := &CRDTSession{
		ID:           sessionID,
		RootPath:     rootPath,
		Documents:    make(map[string]*RGADocument),
		Agents:       make(map[string]*Agent),
		CreatedAt:    time.Now(),
		VectorClock:  NewVectorClock(),
		OperationLog: make([]*Operation, 0),
	}

	ce.sessions[sessionID] = session
	return session
}

// JoinSession allows an agent to join a CRDT session
func (ce *CRDTEngine) JoinSession(sessionID, agentID, agentName string) (*CRDTSession, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	agent := &Agent{
		ID:       agentID,
		Name:     agentName,
		LastSeen: time.Now(),
	}

	session.Agents[agentID] = agent
	return session, nil
}

// GetOrCreateDocument gets or creates a CRDT document for a file
func (ce *CRDTEngine) GetOrCreateDocument(sessionID, filePath, agentID string) (*RGADocument, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	doc, exists := session.Documents[filePath]
	if !exists {
		doc = NewRGADocument(agentID)
		session.Documents[filePath] = doc
	}

	return doc, nil
}

// InitializeDocument initializes a document with content from a file
func (ce *CRDTEngine) InitializeDocument(sessionID, filePath, agentID, content string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Parse content into lines
	lines := parseContentToLines(content)

	// Create or get document
	doc, exists := session.Documents[filePath]
	if !exists {
		doc = NewRGADocument(agentID)
		session.Documents[filePath] = doc
	}

	// Set initial content
	return doc.SetContent(lines)
}

// ApplyOperation applies a CRDT operation to a document
func (ce *CRDTEngine) ApplyOperation(sessionID, filePath string, op *Operation) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	doc, exists := session.Documents[filePath]
	if !exists {
		return fmt.Errorf("document %s not found in session", filePath)
	}

	// Apply operation to document
	if err := doc.ApplyOperation(op); err != nil {
		return fmt.Errorf("failed to apply operation: %w", err)
	}

	// Update session vector clock
	session.VectorClock.Merge(op.VectorClock)

	// Append to operation log for persistence
	session.OperationLog = append(session.OperationLog, op)

	// Update agent last seen
	if agent, exists := session.Agents[op.ReplicaID]; exists {
		agent.LastSeen = time.Now()
	}

	return nil
}

// GetDocumentContent returns the current content of a document
func (ce *CRDTEngine) GetDocumentContent(sessionID, filePath string) (string, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return "", fmt.Errorf("session %s not found", sessionID)
	}

	doc, exists := session.Documents[filePath]
	if !exists {
		return "", fmt.Errorf("document %s not found in session", filePath)
	}

	return doc.GetContentAsString(), nil
}

// GetSession returns a session by ID
func (ce *CRDTEngine) GetSession(sessionID string) (*CRDTSession, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

// ListSessions returns all active sessions
func (ce *CRDTEngine) ListSessions() []*CRDTSession {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	sessions := make([]*CRDTSession, 0, len(ce.sessions))
	for _, session := range ce.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// ImportSession imports a session from external source
func (ce *CRDTEngine) ImportSession(session *CRDTSession) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	ce.sessions[session.ID] = session
}

// SyncOperations synchronizes operations with another node
// Returns operations that the other node doesn't have based on vector clock
func (ce *CRDTEngine) SyncOperations(sessionID string, theirClock *VectorClock) ([]*Operation, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Find operations that the other node hasn't seen
	missingOps := make([]*Operation, 0)

	for _, op := range session.OperationLog {
		// Check if the other node has seen this operation
		theirTimestamp := theirClock.Get(op.ReplicaID)
		opTimestamp := op.VectorClock.Get(op.ReplicaID)

		if opTimestamp > theirTimestamp {
			missingOps = append(missingOps, op)
		}
	}

	return missingOps, nil
}

// ApplyOperations applies multiple operations in order
func (ce *CRDTEngine) ApplyOperations(sessionID, filePath string, ops []*Operation) error {
	for _, op := range ops {
		if err := ce.ApplyOperation(sessionID, filePath, op); err != nil {
			return fmt.Errorf("failed to apply operation: %w", err)
		}
	}
	return nil
}

// GarbageCollect removes tombstones from all documents in a session
func (ce *CRDTEngine) GarbageCollect(sessionID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	totalRemoved := 0
	for _, doc := range session.Documents {
		removed := doc.GarbageCollect(session.VectorClock)
		totalRemoved += removed
	}

	return nil
}

// GetSessionStats returns statistics about a session
type SessionStats struct {
	SessionID       string
	DocumentCount   int
	AgentCount      int
	OperationCount  int
	TotalElements   int
	TotalTombstones int
}

// GetStats returns session statistics
func (ce *CRDTEngine) GetStats(sessionID string) (*SessionStats, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	stats := &SessionStats{
		SessionID:      sessionID,
		DocumentCount:  len(session.Documents),
		AgentCount:     len(session.Agents),
		OperationCount: len(session.OperationLog),
	}

	for _, doc := range session.Documents {
		docStats := doc.GetStats()
		stats.TotalElements += docStats.TotalElements
		stats.TotalTombstones += docStats.Tombstones
	}

	return stats, nil
}

// ToJSON serializes a session to JSON
func (s *CRDTSession) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// parseContentToLines splits content into lines
func parseContentToLines(content string) []string {
	if content == "" {
		return []string{}
	}

	lines := make([]string, 0)
	currentLine := ""

	for _, ch := range content {
		if ch == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(ch)
		}
	}

	// Add last line if it doesn't end with newline
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// CleanupInactiveAgents removes agents that haven't been seen for a while
func (ce *CRDTEngine) CleanupInactiveAgents(timeout time.Duration) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	now := time.Now()
	for _, session := range ce.sessions {
		for agentID, agent := range session.Agents {
			if now.Sub(agent.LastSeen) > timeout {
				delete(session.Agents, agentID)
			}
		}
	}
}

// RemoveAgent removes an agent from a session
func (ce *CRDTEngine) RemoveAgent(sessionID, agentID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	delete(session.Agents, agentID)
	return nil
}

// ListDocuments returns all document paths in a session
func (ce *CRDTEngine) ListDocuments(sessionID string) ([]string, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	paths := make([]string, 0, len(session.Documents))
	for path := range session.Documents {
		paths = append(paths, path)
	}

	return paths, nil
}
