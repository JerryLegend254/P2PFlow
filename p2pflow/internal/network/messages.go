package network

import (
	"encoding/json"

	"github.com/JerryLegend254/p2pflow/internal/collab"
	"github.com/JerryLegend254/p2pflow/internal/crdt"
)

// CRDTMessage represents a CRDT operation message
type CRDTMessage struct {
	Type      CRDTMessageType  `json:"type"`
	SessionID string           `json:"session_id"`
	AgentID   string           `json:"agent_id"`
	Payload   json.RawMessage  `json:"payload"`
	Timestamp int64            `json:"timestamp"`
}

// CRDTMessageType represents the type of CRDT message
type CRDTMessageType int

const (
	// CRDTMessageTypeJoin announces joining a session
	CRDTMessageTypeJoin CRDTMessageType = iota
	// CRDTMessageTypeOperation represents a CRDT operation (insert/delete)
	CRDTMessageTypeOperation
	// CRDTMessageTypeSync requests synchronization with vector clock
	CRDTMessageTypeSync
	// CRDTMessageTypeSyncResponse responds with missing operations
	CRDTMessageTypeSyncResponse
	// CRDTMessageTypePing keeps the connection alive
	CRDTMessageTypePing
	// CRDTMessageTypeAntiEntropy periodic full state synchronization
	CRDTMessageTypeAntiEntropy
	// CRDTMessageTypeFileList sends list of files in session
	CRDTMessageTypeFileList
	// CRDTMessageTypeFileContent sends file content
	CRDTMessageTypeFileContent
)

// CRDTOperationMessage carries a CRDT operation to peers
type CRDTOperationMessage struct {
	FilePath  string          `json:"file_path"`  // Path to the file being modified
	Operation *crdt.Operation `json:"operation"`  // The CRDT operation
}

// CRDTSyncRequest requests operations this node is missing
type CRDTSyncRequest struct {
	SessionID   string             `json:"session_id"`
	AgentID     string             `json:"agent_id"`
	VectorClock *crdt.VectorClock  `json:"vector_clock"` // What this node has seen
}

// CRDTSyncResponse sends operations the requesting node is missing
type CRDTSyncResponse struct {
	SessionID  string                       `json:"session_id"`
	Operations map[string][]*crdt.Operation `json:"operations"` // Map of file path -> operations
}

// CRDTAntiEntropyMessage for periodic full state synchronization
type CRDTAntiEntropyMessage struct {
	SessionID   string                       `json:"session_id"`
	Documents   map[string]*crdt.RGADocument `json:"documents"`    // Full document state
	VectorClock *crdt.VectorClock            `json:"vector_clock"` // Session vector clock
}

// CRDTJoinRequest announces joining a CRDT session
type CRDTJoinRequest struct {
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
}

// CRDTJoinResponse sends session state to newly joined agent
type CRDTJoinResponse struct {
	Session *crdt.CRDTSession `json:"session"`
}

// CRDTFileListMessage sends list of files in the session
type CRDTFileListMessage struct {
	SessionID string   `json:"session_id"`
	FilePaths []string `json:"file_paths"` // Relative paths of files in session
}

// CRDTFileContentMessage sends the actual content of a file
type CRDTFileContentMessage struct {
	SessionID string `json:"session_id"`
	FilePath  string `json:"file_path"` // Relative path
	Content   string `json:"content"`   // File content
}

// Legacy message types (for backward compatibility during migration)

// Message represents a message exchanged between peers (legacy)
type Message struct {
	Type      MessageType     `json:"type"`
	SessionID string          `json:"session_id"`
	AgentID   string          `json:"agent_id"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

// MessageType represents the type of message (legacy)
type MessageType int

const (
	// MessageTypeJoin announces joining a session
	MessageTypeJoin MessageType = iota
	// MessageTypeChange represents a file change/diff
	MessageTypeChange
	// MessageTypeSync requests synchronization
	MessageTypeSync
	// MessageTypeSyncResponse responds to sync request
	MessageTypeSyncResponse
	// MessageTypePing keeps the connection alive
	MessageTypePing
	// MessageTypeFileManifestRequest requests list of all files in session
	MessageTypeFileManifestRequest
	// MessageTypeFileManifestResponse responds with list of files
	MessageTypeFileManifestResponse
	// MessageTypeFileRequest requests content of a specific file
	MessageTypeFileRequest
	// MessageTypeFileTransfer sends file content to peer
	MessageTypeFileTransfer
)

// SyncRequest represents a request for session synchronization (legacy)
type SyncRequest struct {
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
}

// SyncResponse represents a response containing session state (legacy)
type SyncResponse struct {
	Session *collab.Session `json:"session"`
}

// FileManifestRequest requests the list of all files in a session
type FileManifestRequest struct {
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id"`
}

// FileManifestResponse contains the list of files in a session
type FileManifestResponse struct {
	SessionID string                      `json:"session_id"`
	Files     map[string]*collab.FileInfo `json:"files"`
}

// FileRequest requests the content of a specific file
type FileRequest struct {
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id"`
	FilePath  string `json:"file_path"`
}

// FileTransfer contains the content of a file being transferred
type FileTransfer struct {
	SessionID string `json:"session_id"`
	FilePath  string `json:"file_path"`
	Content   string `json:"content"`
	Hash      string `json:"hash"`
	Size      int64  `json:"size"`
}
