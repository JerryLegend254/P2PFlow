package collab

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionManager handles session metadata persistence
type SessionManager struct {
	basePath string
}

// NewSessionManager creates a new session manager
func NewSessionManager(basePath string) *SessionManager {
	return &SessionManager{
		basePath: basePath,
	}
}

// SaveSession saves session metadata to disk
func (sm *SessionManager) SaveSession(session *Session) error {
	// Create .collab directory if it doesn't exist
	collabDir := filepath.Join(sm.basePath, ".collab")
	if err := os.MkdirAll(collabDir, 0755); err != nil {
		return fmt.Errorf("failed to create .collab directory: %v", err)
	}

	// Save session metadata
	sessionFile := filepath.Join(collabDir, fmt.Sprintf("session_%s.json", session.ID))
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %v", err)
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %v", err)
	}

	return nil
}

// LoadSession loads session metadata from disk
func (sm *SessionManager) LoadSession(sessionID string) (*Session, error) {
	sessionFile := filepath.Join(sm.basePath, ".collab", fmt.Sprintf("session_%s.json", sessionID))

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %v", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %v", err)
	}

	return &session, nil
}

// ListSessions returns all saved sessions
func (sm *SessionManager) ListSessions() ([]*Session, error) {
	collabDir := filepath.Join(sm.basePath, ".collab")

	entries, err := os.ReadDir(collabDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, fmt.Errorf("failed to read .collab directory: %v", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionFile := filepath.Join(collabDir, entry.Name())
		data, err := os.ReadFile(sessionFile)
		if err != nil {
			continue // Skip files we can't read
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue // Skip invalid JSON files
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// DeleteSession removes a session from disk
func (sm *SessionManager) DeleteSession(sessionID string) error {
	sessionFile := filepath.Join(sm.basePath, ".collab", fmt.Sprintf("session_%s.json", sessionID))
	return os.Remove(sessionFile)
}

// SaveChangeEvent saves a change event to disk
func (sm *SessionManager) SaveChangeEvent(event *ChangeEvent) error {
	collabDir := filepath.Join(sm.basePath, ".collab")
	if err := os.MkdirAll(collabDir, 0755); err != nil {
		return fmt.Errorf("failed to create .collab directory: %v", err)
	}

	// Create events directory
	eventsDir := filepath.Join(collabDir, "events")
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		return fmt.Errorf("failed to create events directory: %v", err)
	}

	// Save change event with timestamp
	filename := fmt.Sprintf("event_%s_%d.json", event.SessionID, event.Timestamp.Unix())
	eventFile := filepath.Join(eventsDir, filename)

	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal change event: %v", err)
	}

	if err := os.WriteFile(eventFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write event file: %v", err)
	}

	return nil
}

// LoadChangeEvents loads change events for a session
func (sm *SessionManager) LoadChangeEvents(sessionID string) ([]*ChangeEvent, error) {
	eventsDir := filepath.Join(sm.basePath, ".collab", "events")

	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ChangeEvent{}, nil
		}
		return nil, fmt.Errorf("failed to read events directory: %v", err)
	}

	var events []*ChangeEvent
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if this event belongs to our session
		expectedPrefix := fmt.Sprintf("event_%s_", sessionID)
		if !strings.HasPrefix(entry.Name(), expectedPrefix) {
			continue
		}

		eventFile := filepath.Join(eventsDir, entry.Name())
		data, err := os.ReadFile(eventFile)
		if err != nil {
			continue
		}

		var event ChangeEvent
		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}

		events = append(events, &event)
	}

	return events, nil
}

// CleanupOldEvents removes events older than the specified duration
func (sm *SessionManager) CleanupOldEvents(olderThan time.Duration) error {
	eventsDir := filepath.Join(sm.basePath, ".collab", "events")

	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read events directory: %v", err)
	}

	cutoff := time.Now().Add(-olderThan)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Get file modification time
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			eventFile := filepath.Join(eventsDir, entry.Name())
			os.Remove(eventFile)
		}
	}

	return nil
}
