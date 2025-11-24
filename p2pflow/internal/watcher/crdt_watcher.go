package watcher

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/JerryLegend254/p2pflow/internal/crdt"
	"github.com/JerryLegend254/p2pflow/internal/ignore"
	"github.com/fsnotify/fsnotify"
)

// CRDTWatcher watches files and generates CRDT operations
type CRDTWatcher struct {
	path      string
	watcher   *fsnotify.Watcher
	OnChange  func(filePath string, op *crdt.Operation) // Callback when operation is generated

	CRDTEngine  *crdt.CRDTEngine
	SessionID   string
	AgentID     string
	fileStates  map[string]*FileState // Track state per file

	// Track incoming writes to prevent loops
	incomingWrites      map[string]bool
	incomingWritesMutex sync.RWMutex

	// Ignore matcher for filtering files
	IgnoreMatcher *ignore.IgnoreMatcher

	// Debounce handling
	debounceTimer map[string]*time.Timer
	debounceDur   time.Duration
}

// FileState tracks the state of a file for diff generation
type FileState struct {
	Lines    []string          // Current lines in the file
	Document *crdt.RGADocument // CRDT document for this file
}

// NewCRDTWatcher creates a new CRDT-aware file watcher
func NewCRDTWatcher(path string, crdtEngine *crdt.CRDTEngine, sessionID, agentID string) (*CRDTWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Initialize ignore matcher
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	rootPath := absPath
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		rootPath = filepath.Dir(absPath)
	}

	ignoreMatcher := ignore.NewIgnoreMatcher(rootPath)

	watcher := &CRDTWatcher{
		path:           path,
		watcher:        w,
		CRDTEngine:     crdtEngine,
		SessionID:      sessionID,
		AgentID:        agentID,
		fileStates:     make(map[string]*FileState),
		incomingWrites: make(map[string]bool),
		IgnoreMatcher:  ignoreMatcher,
		debounceTimer:  make(map[string]*time.Timer),
		debounceDur:    50 * time.Millisecond, // 50ms debounce
	}

	return watcher, nil
}

// LoadIgnorePatterns loads ignore patterns from configuration and .p2pignore file
func (w *CRDTWatcher) LoadIgnorePatterns(useDefaults bool, useP2PIgnore bool, customPatterns []string) {
	if w.IgnoreMatcher == nil {
		return
	}

	// Add default patterns if enabled
	if useDefaults {
		w.IgnoreMatcher.AddDefaultPatterns()
		fmt.Printf("Loaded default ignore patterns\n")
	}

	// Add custom patterns from config
	for _, pattern := range customPatterns {
		w.IgnoreMatcher.AddPattern(pattern)
	}
	if len(customPatterns) > 0 {
		fmt.Printf("Loaded %d custom ignore patterns from config\n", len(customPatterns))
	}

	// Load .p2pignore file if enabled
	if useP2PIgnore {
		rootPath := w.path
		if info, err := os.Stat(w.path); err == nil && !info.IsDir() {
			rootPath = filepath.Dir(w.path)
		}
		ignoreFile := filepath.Join(rootPath, ".p2pignore")
		if err := w.IgnoreMatcher.LoadFromFile(ignoreFile); err == nil {
			fmt.Printf("Loaded ignore patterns from %s\n", ignoreFile)
		}
	}
}

// InitializeFile initializes the watcher's state for a file
func (w *CRDTWatcher) InitializeFile(filePath string) error {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse content into lines
	lines := parseContentToLines(string(content))

	// Get or create CRDT document
	doc, err := w.CRDTEngine.GetOrCreateDocument(w.SessionID, filePath, w.AgentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Initialize document with content
	if err := doc.SetContent(lines); err != nil {
		return fmt.Errorf("failed to set document content: %w", err)
	}

	// Track file state
	w.fileStates[filePath] = &FileState{
		Lines:    lines,
		Document: doc,
	}

	return nil
}

// Start starts watching for file changes
func (w *CRDTWatcher) Start(errCh chan<- error) error {
	// Check if path is a directory or file
	info, err := os.Stat(w.path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Watch directory - add all files recursively
		if err := w.addDirRecursive(w.path); err != nil {
			return err
		}
		fmt.Printf("CRDT Watching directory: %s\n", w.path)
	} else {
		// Watch single file
		if err := w.watcher.Add(w.path); err != nil {
			return err
		}

		// Initialize file state
		if err := w.InitializeFile(w.path); err != nil {
			fmt.Printf("Warning: Failed to initialize file %s: %v\n", w.path, err)
		}

		fmt.Printf("CRDT Watching file: %s\n", w.path)
	}

	go func() {
		defer w.watcher.Close()
		for {
			select {
			case event := <-w.watcher.Events:
				fmt.Printf("CRDT Event received: %s on %s\n", event.Op, event.Name)

				// Check if file should be ignored
				if w.shouldIgnoreFile(event.Name) {
					fmt.Printf("Ignoring file: %s\n", event.Name)
					continue
				}

				// Handle different event types
				if event.Op&fsnotify.Write == fsnotify.Write {
					w.debounceHandleWrite(event.Name)
				} else if event.Op&fsnotify.Create == fsnotify.Create {
					w.handleCreate(event.Name)
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					w.handleRemove(event.Name)
				}

			case err := <-w.watcher.Errors:
				if errCh != nil {
					errCh <- err
				}
			}
		}
	}()

	return nil
}

// debounceHandleWrite debounces write events to prevent excessive operations
func (w *CRDTWatcher) debounceHandleWrite(filePath string) {
	// Cancel existing timer for this file
	if timer, exists := w.debounceTimer[filePath]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceTimer[filePath] = time.AfterFunc(w.debounceDur, func() {
		w.handleWrite(filePath)
		delete(w.debounceTimer, filePath)
	})
}

// handleWrite handles file write events and generates CRDT operations
func (w *CRDTWatcher) handleWrite(filePath string) {
	// Normalize path to absolute for consistent tracking
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	// Check if this is an incoming write (from remote peer)
	w.incomingWritesMutex.RLock()
	isIncoming := w.incomingWrites[absPath]
	w.incomingWritesMutex.RUnlock()

	if isIncoming {
		// This is a write from ApplyRemoteOperation, skip to prevent loop
		w.incomingWritesMutex.Lock()
		delete(w.incomingWrites, absPath)
		w.incomingWritesMutex.Unlock()
		fmt.Printf("Skipping incoming write for: %s (tracked as %s)\n", filePath, absPath)

		// Still need to update our file state
		content, err := os.ReadFile(filePath)
		if err == nil {
			newLines := parseContentToLines(string(content))
			if state, exists := w.fileStates[filePath]; exists {
				state.Lines = newLines
			}
		}
		return
	}

	fmt.Printf("→ Processing local write for: %s (tracked as %s)\n", filePath, absPath)

	// Read new content
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", filePath, err)
		return
	}

	// Parse content into lines
	newLines := parseContentToLines(string(content))

	// Get file state
	state, exists := w.fileStates[filePath]
	if !exists {
		// File not initialized, initialize it
		if err := w.InitializeFile(filePath); err != nil {
			fmt.Printf("Error initializing file %s: %v\n", filePath, err)
			return
		}
		state = w.fileStates[filePath]
	}

	oldLines := state.Lines

	// Generate CRDT operations from line diff
	operations := w.generateOperations(filePath, oldLines, newLines)

	fmt.Printf("  Generated %d operations for %s (old: %d lines, new: %d lines)\n",
		len(operations), filePath, len(oldLines), len(newLines))

	// Apply operations locally and broadcast
	for i, op := range operations {
		// Apply operation to CRDT engine
		if err := w.CRDTEngine.ApplyOperation(w.SessionID, filePath, op); err != nil {
			fmt.Printf("Error applying operation: %v\n", err)
			continue
		}

		// Trigger callback to broadcast operation
		if w.OnChange != nil {
			fmt.Printf("  Broadcasting operation %d/%d for %s: type=%s, content=%q\n",
				i+1, len(operations), filePath, op.Type, op.Content)
			w.OnChange(filePath, op)
		}
	}

	// Update file state
	state.Lines = newLines
	fmt.Printf("Completed processing write for: %s\n", filePath)
}

// handleCreate handles file creation events
func (w *CRDTWatcher) handleCreate(filePath string) {
	info, err := os.Stat(filePath)
	if err != nil {
		return
	}

	if info.IsDir() {
		// Add directory to watcher
		w.addDirRecursive(filePath)
	} else {
		// Add file to watcher
		w.watcher.Add(filePath)

		// Initialize file
		if err := w.InitializeFile(filePath); err != nil {
			fmt.Printf("Error initializing new file %s: %v\n", filePath, err)
		}
	}
}

// handleRemove handles file removal events
func (w *CRDTWatcher) handleRemove(filePath string) {
	// Remove file state
	delete(w.fileStates, filePath)

	// TODO: Generate delete operations for all lines in the file
	// For now, we'll just remove the file from tracking
}

// generateOperations generates CRDT operations from line diff
func (w *CRDTWatcher) generateOperations(filePath string, oldLines, newLines []string) []*crdt.Operation {
	operations := make([]*crdt.Operation, 0)

	state := w.fileStates[filePath]
	if state == nil || state.Document == nil {
		return operations
	}

	doc := state.Document

	// Use a simple LCS-based diff algorithm
	// This computes the minimum set of insertions and deletions

	oldLen := len(oldLines)
	newLen := len(newLines)

	// Build LCS table
	lcs := make([][]int, oldLen+1)
	for i := range lcs {
		lcs[i] = make([]int, newLen+1)
	}

	for i := 1; i <= oldLen; i++ {
		for j := 1; j <= newLen; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				if lcs[i-1][j] > lcs[i][j-1] {
					lcs[i][j] = lcs[i-1][j]
				} else {
					lcs[i][j] = lcs[i][j-1]
				}
			}
		}
	}

	// Backtrack to find operations
	// We need to track current position as we apply operations
	i, j := oldLen, newLen
	var tempOps []struct {
		isDelete bool
		index    int
		content  string
	}

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			// Lines are the same, no operation needed
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			// Insert operation
			tempOps = append(tempOps, struct {
				isDelete bool
				index    int
				content  string
			}{false, j - 1, newLines[j-1]})
			j--
		} else if i > 0 {
			// Delete operation
			tempOps = append(tempOps, struct {
				isDelete bool
				index    int
				content  string
			}{true, i - 1, ""})
			i--
		}
	}

	// Reverse operations (we built them backwards)
	for i := len(tempOps) - 1; i >= 0; i-- {
		op := tempOps[i]
		if op.isDelete {
			// For delete, we need to find the actual position in the current document
			deleteOp, err := doc.Delete(op.index)
			if err == nil {
				operations = append(operations, deleteOp)
			}
		} else {
			// For insert
			insertOp, err := doc.Insert(op.index, op.content)
			if err == nil {
				operations = append(operations, insertOp)
			}
		}
	}

	return operations
}

// addDirRecursive adds a directory and all its subdirectories to the watcher
func (w *CRDTWatcher) addDirRecursive(path string) error {
	return filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if file should be ignored
		if w.shouldIgnoreFile(walkPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			if err := w.watcher.Add(walkPath); err != nil {
				return err
			}
		} else {
			// Initialize file state
			if err := w.InitializeFile(walkPath); err != nil {
				fmt.Printf("Warning: Failed to initialize file %s: %v\n", walkPath, err)
			}
		}

		return nil
	})
}

// shouldIgnoreFile checks if a file should be ignored based on patterns
func (w *CRDTWatcher) shouldIgnoreFile(filePath string) bool {
	if w.IgnoreMatcher == nil {
		return false
	}

	// Get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	// Determine if it's a directory
	isDir := false
	if info, err := os.Stat(absPath); err == nil {
		isDir = info.IsDir()
	}

	return w.IgnoreMatcher.ShouldIgnore(absPath, isDir)
}

// Close closes the watcher
func (w *CRDTWatcher) Close() error {
	// Cancel all pending timers
	for _, timer := range w.debounceTimer {
		timer.Stop()
	}

	return w.watcher.Close()
}

// parseContentToLines splits content into lines
func parseContentToLines(content string) []string {
	if content == "" {
		return []string{}
	}

	lines := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines
}

// generateCRDTAgentID generates a random agent ID for CRDT watchers
func generateCRDTAgentID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetFileContent returns the current content of a file being watched
func (w *CRDTWatcher) GetFileContent(filePath string) (string, error) {
	state, exists := w.fileStates[filePath]
	if !exists {
		return "", fmt.Errorf("file %s not being watched", filePath)
	}

	return strings.Join(state.Lines, "\n"), nil
}

// ApplyRemoteOperation applies an operation from a remote peer
func (w *CRDTWatcher) ApplyRemoteOperation(filePath string, op *crdt.Operation) error {
	// Apply operation to CRDT engine
	if err := w.CRDTEngine.ApplyOperation(w.SessionID, filePath, op); err != nil {
		return fmt.Errorf("failed to apply remote operation: %w", err)
	}

	// Get updated content from CRDT document
	content, err := w.CRDTEngine.GetDocumentContent(w.SessionID, filePath)
	if err != nil {
		return fmt.Errorf("failed to get document content: %w", err)
	}

	// Resolve full path - filePath might be relative
	fullPath := filePath
	if !filepath.IsAbs(filePath) {
		fullPath = filepath.Join(w.path, filePath)
	}

	// Get absolute path for consistent tracking
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		absPath = fullPath
	}

	// Mark this as an incoming write BEFORE writing to prevent loop
	w.incomingWritesMutex.Lock()
	w.incomingWrites[absPath] = true
	w.incomingWritesMutex.Unlock()

	// Write content to file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		// Clean up the flag if write fails
		w.incomingWritesMutex.Lock()
		delete(w.incomingWrites, absPath)
		w.incomingWritesMutex.Unlock()
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Note: file state will be updated by handleWrite after it processes the event

	return nil
}

// MarkIncomingWrite marks a file path as an incoming write to prevent loop
// This should be called before writing a file that comes from a remote source
// (e.g., initial sync, remote operations) to prevent the watcher from generating
// operations for content that was not locally edited.
func (w *CRDTWatcher) MarkIncomingWrite(filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	w.incomingWritesMutex.Lock()
	w.incomingWrites[absPath] = true
	w.incomingWritesMutex.Unlock()

	fmt.Printf("Marked as incoming write: %s (tracked as %s)\n", filePath, absPath)
}
