package watcher

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JerryLegend254/p2pflow/internal/collab"
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
}

func NewWatcher(path string) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Initialize collaboration engine
	collabEngine := collab.NewCollaborationEngine()
	sessionManager := collab.NewSessionManager(".")

	d := &Watcher{
		path:           path,
		watcher:        w,
		Dmp:            dmp.New(),
		CollabEngine:   collabEngine,
		SessionManager: sessionManager,
		SessionID:      generateSessionID(),
		AgentID:        generateAgentID(),
		fileContents:   make(map[string]string),
	}

	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		b, _ := os.ReadFile(path)
		d.last = string(b)
		fmt.Printf("Initial content loaded: %d bytes\n", len(d.last))

	}
	return d, nil
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

		// Skip hidden files and directories
		if info.IsDir() && info.Name()[0] == '.' {
			return filepath.SkipDir
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
