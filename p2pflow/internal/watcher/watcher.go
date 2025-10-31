package watcher

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
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
	OnChange func(patch string)

	CollabEngine   *collab.CollaborationEngine
	SessionManager *collab.SessionManager
	SessionID      string
	AgentID        string
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
	}

	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		b, _ := os.ReadFile(path)
		d.last = string(b)
		fmt.Printf("Initial content loaded: %d bytes\n", len(d.last))

		// Create collaboration session
		session := d.CollabEngine.CreateSession(d.SessionID, path, d.last)
		d.SessionManager.SaveSession(session)
		fmt.Printf("Created collaboration session: %s\n", d.SessionID)
	}
	return d, nil
}

func (w *Watcher) Start(errCh chan<- error) error {
	if err := w.watcher.Add(w.path); err != nil {
		return err
	}

	fmt.Printf("Watching: %s\n", w.path)

	// TODO: watch directory
	// watch single file for now for simplicity due to patches
	go func() {
		defer w.watcher.Close()
		for {
			select {
			case event := <-w.watcher.Events:
				fmt.Printf("Event received: %s on %s\n", event.Op, event.Name)
				// Only process write events for files we're watching
				if event.Op&fsnotify.Write == fsnotify.Write && event.Name == w.path {
					fmt.Printf("Write detected: %s\n", event.Name)

					// Small debounce to avoid excessive patch generation
					time.Sleep(50 * time.Millisecond)

					// Read current file content
					b, err := os.ReadFile(w.path)
					if err != nil {
						fmt.Printf("Error reading file: %v\n", err)
						continue
					}

					cur := string(b)

					// Skip if content hasn't changed
					if cur == w.last {
						continue
					}

					diffs := w.Dmp.DiffMain(w.last, cur, false)
					patch := w.Dmp.PatchToText(w.Dmp.PatchMake(diffs))

					w.last = cur

					changeEvent := &collab.ChangeEvent{
						SessionID: w.SessionID,
						AgentID:   w.AgentID,
						Timestamp: time.Now(),
						Patch:     patch,
						Version:   0, // Will be set by the collaboration engine
					}

					// Apply change to collaboration engine
					session, err := w.CollabEngine.ApplyChange(changeEvent)
					if err != nil {
						fmt.Printf("Error applying change: %v\n", err)
					} else {
						// Save change event to disk
						w.SessionManager.SaveChangeEvent(changeEvent)
						// Update session
						w.SessionManager.SaveSession(session)
						fmt.Printf("Applied change to session %s, version %d\n", session.ID, session.Version)
					}

					if w.OnChange != nil {
						w.OnChange(patch)
					}

					fmt.Printf("Generated patch: %s\n", patch)
				} else {
					switch {
					case event.Op&fsnotify.Create == fsnotify.Create:
						fmt.Printf("Create: %s: %s\n", event.Op, event.Name)
					case event.Op&fsnotify.Remove == fsnotify.Remove:
						fmt.Printf("Remove: %s: %s\n", event.Op, event.Name)
					case event.Op&fsnotify.Rename == fsnotify.Rename:
						fmt.Printf("Rename: %s: %s\n", event.Op, event.Name)
					case event.Op&fsnotify.Chmod == fsnotify.Chmod:
						fmt.Printf("Chmod:  %s: %s\n", event.Op, event.Name)
					}
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
