package watcher

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JerryLegend254/p2pflow/internal/analytics"
	"github.com/JerryLegend254/p2pflow/internal/collab"
	"github.com/JerryLegend254/p2pflow/internal/ignore"
	"github.com/fsnotify/fsnotify"
	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

type ChangeEvent struct {
	SessionID string `json:"session_id"`
	User      string `json:"user"`
	Timestamp int64  `json:"ts"`
	Patch     string `json:"patch"` // diffpatch
}
type Watcher struct {
	path     string
	watcher  *fsnotify.Watcher
	last     string
	Dmp      *dmp.DiffMatchPatch
	OnChange func(patch string, filePath string)  // Updated to include filePath

	CollabEngine   *collab.CollaborationEngine
	SessionManager *collab.SessionManager
	SessionID      string
	AgentID        string
	fileContents   map[string]string  // Track content per file for multi-file watching

	// Callback to check if a file write is from an incoming change
	IsIncomingWrite func(filePath string) bool

	// Ignore matcher for filtering files (exported for sharing with P2P node)
	IgnoreMatcher *ignore.IgnoreMatcher

	// Analytics engine for tracking file access patterns
	AnalyticsEngine *analytics.AnalyticsEngine
}

func NewWatcher(path string) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Initialize collaboration engine
	collabEngine := collab.NewCollaborationEngine()
	sessionManager := collab.NewSessionManager(".")

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

	d := &Watcher{
		path:           path,
		watcher:        w,
		Dmp:            dmp.New(),
		CollabEngine:   collabEngine,
		SessionManager: sessionManager,
		SessionID:      generateSessionID(),
		AgentID:        generateAgentID(),
		fileContents:   make(map[string]string),
		IgnoreMatcher:  ignoreMatcher,
	}

	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		b, _ := os.ReadFile(path)
		d.last = string(b)
		fmt.Printf("Initial content loaded: %d bytes\n", len(d.last))

	}
	return d, nil
}

// LoadIgnorePatterns loads ignore patterns from configuration and .p2pignore file
func (w *Watcher) LoadIgnorePatterns(useDefaults bool, useP2PIgnore bool, customPatterns []string) {
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

func (w *Watcher) Start(errCh chan<- error) error {
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
		fmt.Printf("Watching directory: %s\n", w.path)
	} else {
		// Watch single file
		if err := w.watcher.Add(w.path); err != nil {
			return err
		}
		fmt.Printf("Watching file: %s\n", w.path)
	}

	go func() {
		defer w.watcher.Close()
		for {
			select {
			case event := <-w.watcher.Events:
				fmt.Printf("Event received: %s on %s\n", event.Op, event.Name)

				// Handle different event types
				switch {
				case event.Op&fsnotify.Write == fsnotify.Write:
					w.handleFileWrite(event.Name)
				case event.Op&fsnotify.Create == fsnotify.Create:
					w.handleFileCreate(event.Name)
				case event.Op&fsnotify.Remove == fsnotify.Remove:
					fmt.Printf("Remove: %s\n", event.Name)
					// TODO: Broadcast file deletion
				case event.Op&fsnotify.Rename == fsnotify.Rename:
					fmt.Printf("Rename: %s\n", event.Name)
					// TODO: Broadcast file rename
				case event.Op&fsnotify.Chmod == fsnotify.Chmod:
					fmt.Printf("Chmod: %s\n", event.Name)
					// Ignore permission changes
				}
			case err := <-w.watcher.Errors:
				if err != nil {
					errCh <- err
				}
			}
		}
	}()

	return nil
}

func generateSessionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateAgentID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// addDirRecursive adds a directory and all its subdirectories to the watcher
func (w *Watcher) addDirRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if path should be ignored
		if w.IgnoreMatcher != nil && w.IgnoreMatcher.ShouldIgnore(path, info.IsDir()) {
			if info.IsDir() {
				fmt.Printf("  Skipping ignored directory: %s\n", path)
				return filepath.SkipDir
			}
			fmt.Printf("  Skipping ignored file: %s\n", path)
			return nil
		}

		// Add directories to watcher
		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				return err
			}
			fmt.Printf("  Added to watch: %s\n", path)
		}

		return nil
	})
}

// handleFileWrite handles write events to files
func (w *Watcher) handleFileWrite(filePath string) {
	fmt.Printf("Write detected: %s\n", filePath)

	// Check if file should be ignored
	if w.IgnoreMatcher != nil {
		info, err := os.Stat(filePath)
		isDir := err == nil && info.IsDir()
		if w.IgnoreMatcher.ShouldIgnore(filePath, isDir) {
			fmt.Printf("Ignoring write to filtered file: %s\n", filePath)
			return
		}
	}

	// Check if this is an incoming write from a peer (to prevent loops)
	if w.IsIncomingWrite != nil && w.IsIncomingWrite(filePath) {
		fmt.Printf("Skipping write event for %s (incoming change from peer)\n", filePath)

		// Still update our local content cache to stay in sync
		b, _ := os.ReadFile(filePath)
		if b != nil {
			w.fileContents[filePath] = string(b)
			w.last = string(b)
		}
		return
	}

	// Record analytics: local file write
	if w.AnalyticsEngine != nil {
		w.AnalyticsEngine.RecordFileAccess(filePath, analytics.AccessTypeWrite)
	}

	// Small debounce to avoid excessive patch generation
	time.Sleep(50 * time.Millisecond)

	// Read current file content
	b, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	cur := string(b)

	// Get previous content for this file
	prev, exists := w.fileContents[filePath]
	if !exists {
		prev = w.last  // Fallback to single-file mode
	}

	// Skip if content hasn't changed
	if cur == prev {
		return
	}

	// Generate diff
	diffs := w.Dmp.DiffMain(prev, cur, false)
	patch := w.Dmp.PatchToText(w.Dmp.PatchMake(diffs))

	// Update stored content
	w.fileContents[filePath] = cur
	w.last = cur  // For backward compatibility

	if w.OnChange != nil {
		w.OnChange(patch, filePath)  // Pass file path
	}

	fmt.Printf("Generated patch for %s: %s\n", filePath, patch)
}

// handleFileCreate handles creation of new files or directories
func (w *Watcher) handleFileCreate(filePath string) {
	fmt.Printf("Create: %s\n", filePath)

	// Check if it's a directory
	info, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("Error stating file: %v\n", err)
		return
	}

	// Check if file/directory should be ignored
	if w.IgnoreMatcher != nil && w.IgnoreMatcher.ShouldIgnore(filePath, info.IsDir()) {
		fmt.Printf("Ignoring creation of filtered file/directory: %s\n", filePath)
		return
	}

	// Record analytics: file creation
	if w.AnalyticsEngine != nil && !info.IsDir() {
		w.AnalyticsEngine.RecordFileAccess(filePath, analytics.AccessTypeCreate)
	}

	if info.IsDir() {
		// Add new directory to watcher
		if err := w.watcher.Add(filePath); err != nil {
			fmt.Printf("Error adding directory to watcher: %v\n", err)
		} else {
			fmt.Printf("Added new directory to watch: %s\n", filePath)
		}
	} else {
		// New file created - treat as a write with empty previous content
		b, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error reading new file: %v\n", err)
			return
		}

		content := string(b)
		diffs := w.Dmp.DiffMain("", content, false)
		patch := w.Dmp.PatchToText(w.Dmp.PatchMake(diffs))

		// Store initial content
		w.fileContents[filePath] = content

		if w.OnChange != nil {
			w.OnChange(patch, filePath)  // Pass file path
		}

		fmt.Printf("Generated patch for new file %s: %s\n", filePath, patch)
	}
}
