package collab

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

// FileInfo represents metadata about a file in the session
type FileInfo struct {
	Path         string    `json:"path"`          // Relative path from RootPath
	Hash         string    `json:"hash"`          // SHA256 hash for integrity
	Size         int64     `json:"size"`          // File size in bytes
	LastModified time.Time `json:"last_modified"` // Last modification time
	Content      string    `json:"content"`       // Current file content
	Version      int       `json:"version"`       // File-specific version
}

// Session represents a collaboration session
type Session struct {
	ID        string               `json:"id"`
	FilePath  string               `json:"file_path"`  // Deprecated: kept for backward compatibility
	RootPath  string               `json:"root_path"`  // Base directory for the session
	Files     map[string]*FileInfo `json:"files"`      // Map of relative path -> FileInfo
	CreatedAt time.Time            `json:"created_at"`
	Agents    map[string]*Agent    `json:"agents"`
	Content   string               `json:"content"`    // Deprecated: kept for backward compatibility
	Version   int                  `json:"version"`
}

// Agent represents a participant in the collaboration
type Agent struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	LastSeen time.Time `json:"last_seen"`
	Version  int       `json:"version"`
}

// ChangeEvent represents a change made by an agent
type ChangeEvent struct {
	SessionID string    `json:"session_id"`
	AgentID   string    `json:"agent_id"`
	Timestamp time.Time `json:"timestamp"`
	Patch     string    `json:"patch"`
	Version   int       `json:"version"`
}

// CollaborationEngine manages shared state and patch merging
type CollaborationEngine struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	dmp      *dmp.DiffMatchPatch
}

// NewCollaborationEngine creates a new collaboration engine
func NewCollaborationEngine() *CollaborationEngine {
	return &CollaborationEngine{
		sessions: make(map[string]*Session),
		dmp:      dmp.New(),
	}
}

// CreateSession creates a new collaboration session
// For backward compatibility, it accepts a single file path and content
// For multi-file sessions, use CreateSessionWithFiles instead
func (ce *CollaborationEngine) CreateSession(sessionID, filePath, content string) *Session {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session := &Session{
		ID:        sessionID,
		FilePath:  filePath,  // Backward compatibility
		RootPath:  "",
		Files:     make(map[string]*FileInfo),
		CreatedAt: time.Now(),
		Agents:    make(map[string]*Agent),
		Content:   content,   // Backward compatibility
		Version:   0,
	}

	// If a file path is provided, add it as the first file
	if filePath != "" && content != "" {
		session.Files[filePath] = &FileInfo{
			Path:         filePath,
			Content:      content,
			Size:         int64(len(content)),
			LastModified: time.Now(),
			Version:      0,
		}
	}

	ce.sessions[sessionID] = session
	return session
}

// JoinSession allows an agent to join a session
func (ce *CollaborationEngine) JoinSession(sessionID, agentID, agentName string) (*Session, error) {
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
		Version:  session.Version,
	}

	session.Agents[agentID] = agent
	return session, nil
}

// ApplyChange applies a change event to the session
func (ce *CollaborationEngine) ApplyChange(event *ChangeEvent) (*Session, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[event.SessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", event.SessionID)
	}

	// Check if agent exists in session
	agent, exists := session.Agents[event.AgentID]
	if !exists {
		return nil, fmt.Errorf("agent %s not found in session", event.AgentID)
	}

	// Update agent's last seen time
	agent.LastSeen = time.Now()

	// Apply the patch to current content
	patches, err := ce.dmp.PatchFromText(event.Patch)
	if err != nil {
		return nil, fmt.Errorf("invalid patch: %v", err)
	}

	// Apply patches to get new content
	newContent, results := ce.dmp.PatchApply(patches, session.Content)
	if !results[0] {
		return nil, fmt.Errorf("failed to apply patch")
	}

	// Update session content and version
	session.Content = newContent
	session.Version++
	agent.Version = session.Version

	return session, nil
}

// MergeChanges attempts to merge changes from multiple agents
func (ce *CollaborationEngine) MergeChanges(sessionID string, changes []*ChangeEvent) (*Session, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Sort changes by timestamp to apply in order
	// For now, we'll apply changes sequentially
	// In a more sophisticated implementation, we'd use operational transforms or CRDTs

	for _, change := range changes {
		patches, err := ce.dmp.PatchFromText(change.Patch)
		if err != nil {
			continue // Skip invalid patches
		}

		// Try to apply the patch
		newContent, results := ce.dmp.PatchApply(patches, session.Content)
		if results[0] {
			session.Content = newContent
			session.Version++
		}
		// If patch application fails, we could implement conflict resolution here
	}

	return session, nil
}

// GetSession returns a session by ID
func (ce *CollaborationEngine) GetSession(sessionID string) (*Session, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

// ImportSession imports a session from external source (e.g., from peer sync)
func (ce *CollaborationEngine) ImportSession(session *Session) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	ce.sessions[session.ID] = session
}

// ListSessions returns all active sessions
func (ce *CollaborationEngine) ListSessions() []*Session {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	sessions := make([]*Session, 0, len(ce.sessions))
	for _, session := range ce.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// RemoveAgent removes an agent from a session
func (ce *CollaborationEngine) RemoveAgent(sessionID, agentID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	delete(session.Agents, agentID)
	return nil
}

// CleanupInactiveAgents removes agents that haven't been seen for a while
func (ce *CollaborationEngine) CleanupInactiveAgents(timeout time.Duration) {
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

// ToJSON serializes a session to JSON
func (s *Session) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// ToJSON serializes a change event to JSON
func (ce *ChangeEvent) ToJSON() ([]byte, error) {
	return json.Marshal(ce)
}

// AddFile adds a file to the session
func (ce *CollaborationEngine) AddFile(sessionID, filePath, content string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.Files[filePath] = &FileInfo{
		Path:         filePath,
		Content:      content,
		Size:         int64(len(content)),
		LastModified: time.Now(),
		Version:      0,
	}

	return nil
}

// GetFile retrieves a file from the session
func (ce *CollaborationEngine) GetFile(sessionID, filePath string) (*FileInfo, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	file, exists := session.Files[filePath]
	if !exists {
		return nil, fmt.Errorf("file %s not found in session", filePath)
	}

	return file, nil
}

// ListFiles returns all files in a session
func (ce *CollaborationEngine) ListFiles(sessionID string) (map[string]*FileInfo, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session.Files, nil
}

// UpdateFileContent updates the content of a file in the session
func (ce *CollaborationEngine) UpdateFileContent(sessionID, filePath, newContent string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	session, exists := ce.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	file, exists := session.Files[filePath]
	if !exists {
		return fmt.Errorf("file %s not found in session", filePath)
	}

	file.Content = newContent
	file.Size = int64(len(newContent))
	file.LastModified = time.Now()
	file.Version++

	return nil
}
