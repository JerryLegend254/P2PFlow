package crdt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SessionPersistence handles saving and loading CRDT sessions
type SessionPersistence struct {
	baseDir string
}

// NewSessionPersistence creates a new session persistence manager
func NewSessionPersistence(baseDir string) *SessionPersistence {
	return &SessionPersistence{
		baseDir: baseDir,
	}
}

// SaveSession saves a CRDT session to disk
func (sp *SessionPersistence) SaveSession(session *CRDTSession) error {
	// Create base directory if it doesn't exist
	sessionDir := filepath.Join(sp.baseDir, ".crdt")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create session file path
	sessionFile := filepath.Join(sessionDir, fmt.Sprintf("session_%s.json", session.ID))

	// Serialize session to JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to file
	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads a CRDT session from disk
func (sp *SessionPersistence) LoadSession(sessionID string) (*CRDTSession, error) {
	sessionFile := filepath.Join(sp.baseDir, ".crdt", fmt.Sprintf("session_%s.json", sessionID))

	// Read session file
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Deserialize session
	var session CRDTSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// SaveOperation saves a single operation to the operation log
func (sp *SessionPersistence) SaveOperation(sessionID string, op *Operation) error {
	// Create operations directory
	opsDir := filepath.Join(sp.baseDir, ".crdt", "operations", sessionID)
	if err := os.MkdirAll(opsDir, 0755); err != nil {
		return fmt.Errorf("failed to create operations directory: %w", err)
	}

	// Create operation file path with timestamp
	opFile := filepath.Join(opsDir, fmt.Sprintf("op_%d_%s.json", op.Timestamp, op.ReplicaID))

	// Serialize operation to JSON
	data, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal operation: %w", err)
	}

	// Write to file
	if err := os.WriteFile(opFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write operation file: %w", err)
	}

	return nil
}

// LoadOperations loads all operations for a session
func (sp *SessionPersistence) LoadOperations(sessionID string) ([]*Operation, error) {
	opsDir := filepath.Join(sp.baseDir, ".crdt", "operations", sessionID)

	// Check if directory exists
	if _, err := os.Stat(opsDir); os.IsNotExist(err) {
		return []*Operation{}, nil
	}

	// Read all operation files
	files, err := os.ReadDir(opsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read operations directory: %w", err)
	}

	operations := make([]*Operation, 0, len(files))

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Read operation file
		opFile := filepath.Join(opsDir, file.Name())
		data, err := os.ReadFile(opFile)
		if err != nil {
			continue
		}

		// Deserialize operation
		var op Operation
		if err := json.Unmarshal(data, &op); err != nil {
			continue
		}

		operations = append(operations, &op)
	}

	return operations, nil
}

// DeleteSession deletes a session and all its operations
func (sp *SessionPersistence) DeleteSession(sessionID string) error {
	// Delete session file
	sessionFile := filepath.Join(sp.baseDir, ".crdt", fmt.Sprintf("session_%s.json", sessionID))
	if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	// Delete operations directory
	opsDir := filepath.Join(sp.baseDir, ".crdt", "operations", sessionID)
	if err := os.RemoveAll(opsDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete operations directory: %w", err)
	}

	return nil
}

// ListSessions lists all saved sessions
func (sp *SessionPersistence) ListSessions() ([]string, error) {
	sessionDir := filepath.Join(sp.baseDir, ".crdt")

	// Check if directory exists
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	// Read all session files
	files, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	sessions := make([]string, 0)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Extract session ID from filename
		name := file.Name()
		if len(name) > 8 && name[:8] == "session_" && name[len(name)-5:] == ".json" {
			sessionID := name[8 : len(name)-5]
			sessions = append(sessions, sessionID)
		}
	}

	return sessions, nil
}

// SaveSnapshot saves a complete snapshot of a session
type Snapshot struct {
	Session    *CRDTSession `json:"session"`
	Operations []*Operation `json:"operations"`
	CreatedAt  time.Time    `json:"created_at"`
}

// CreateSnapshot creates a snapshot of a session
func (sp *SessionPersistence) CreateSnapshot(session *CRDTSession, operations []*Operation) error {
	// Create snapshots directory
	snapshotDir := filepath.Join(sp.baseDir, ".crdt", "snapshots")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Create snapshot
	snapshot := Snapshot{
		Session:    session,
		Operations: operations,
		CreatedAt:  time.Now(),
	}

	// Create snapshot file path with timestamp
	timestamp := time.Now().Unix()
	snapshotFile := filepath.Join(snapshotDir, fmt.Sprintf("snapshot_%s_%d.json", session.ID, timestamp))

	// Serialize snapshot to JSON
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Write to file
	if err := os.WriteFile(snapshotFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}

// LoadLatestSnapshot loads the most recent snapshot for a session
func (sp *SessionPersistence) LoadLatestSnapshot(sessionID string) (*Snapshot, error) {
	snapshotDir := filepath.Join(sp.baseDir, ".crdt", "snapshots")

	// Check if directory exists
	if _, err := os.Stat(snapshotDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("no snapshots found")
	}

	// Read all snapshot files
	files, err := os.ReadDir(snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshots directory: %w", err)
	}

	// Find latest snapshot for this session
	var latestFile string
	var latestTime int64

	prefix := fmt.Sprintf("snapshot_%s_", sessionID)
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if len(name) > len(prefix) && name[:len(prefix)] == prefix {
			// Extract timestamp from filename
			var timestamp int64
			fmt.Sscanf(name[len(prefix):], "%d.json", &timestamp)

			if timestamp > latestTime {
				latestTime = timestamp
				latestFile = filepath.Join(snapshotDir, name)
			}
		}
	}

	if latestFile == "" {
		return nil, fmt.Errorf("no snapshots found for session %s", sessionID)
	}

	// Read snapshot file
	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	// Deserialize snapshot
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}

// CleanupOldSnapshots removes snapshots older than the specified duration
func (sp *SessionPersistence) CleanupOldSnapshots(maxAge time.Duration) error {
	snapshotDir := filepath.Join(sp.baseDir, ".crdt", "snapshots")

	// Check if directory exists
	if _, err := os.Stat(snapshotDir); os.IsNotExist(err) {
		return nil
	}

	// Read all snapshot files
	files, err := os.ReadDir(snapshotDir)
	if err != nil {
		return fmt.Errorf("failed to read snapshots directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge).Unix()

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Extract timestamp from filename
		var timestamp int64
		fmt.Sscanf(file.Name(), "snapshot_%*s_%d.json", &timestamp)

		if timestamp < cutoff {
			// Delete old snapshot
			snapshotFile := filepath.Join(snapshotDir, file.Name())
			if err := os.Remove(snapshotFile); err != nil {
				fmt.Printf("Warning: Failed to delete old snapshot %s: %v\n", file.Name(), err)
			}
		}
	}

	return nil
}

// GetStorageStats returns storage statistics for CRDT data
type StorageStats struct {
	SessionCount   int
	OperationCount int
	SnapshotCount  int
	TotalSize      int64
}

// GetStats returns storage statistics
func (sp *SessionPersistence) GetStats() (*StorageStats, error) {
	stats := &StorageStats{}

	crdtDir := filepath.Join(sp.baseDir, ".crdt")

	// Check if directory exists
	if _, err := os.Stat(crdtDir); os.IsNotExist(err) {
		return stats, nil
	}

	// Count sessions
	sessionFiles, err := os.ReadDir(crdtDir)
	if err == nil {
		for _, file := range sessionFiles {
			if !file.IsDir() && len(file.Name()) > 8 && file.Name()[:8] == "session_" {
				stats.SessionCount++
			}
		}
	}

	// Count operations
	opsDir := filepath.Join(crdtDir, "operations")
	if opsDirs, err := os.ReadDir(opsDir); err == nil {
		for _, sessionDir := range opsDirs {
			if sessionDir.IsDir() {
				if ops, err := os.ReadDir(filepath.Join(opsDir, sessionDir.Name())); err == nil {
					stats.OperationCount += len(ops)
				}
			}
		}
	}

	// Count snapshots
	snapshotDir := filepath.Join(crdtDir, "snapshots")
	if snapshots, err := os.ReadDir(snapshotDir); err == nil {
		stats.SnapshotCount = len(snapshots)
	}

	// Calculate total size
	filepath.Walk(crdtDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			stats.TotalSize += info.Size()
		}
		return nil
	})

	return stats, nil
}
