package main

import (
	"fmt"
	"os"

	"github.com/JerryLegend254/p2pflow/internal/logger"
	"github.com/JerryLegend254/p2pflow/internal/watcher"
)

func main() {
	jsonLogger := logger.NewLogger(logger.JSON)
	jsonLogger.Sync()

	fmt.Println("Testing P2P File Watcher with Collaboration Engine")
	fmt.Println("======================================================")

	cleanupTestFiles()

	testFile := "test-watcher.txt"
	initialContent := "Hello, this is a test file for watcher collaboration.\nInitial content.\n"

	err := os.WriteFile(testFile, []byte(initialContent), 0644)
	if err != nil {
		jsonLogger.Fatalf("Failed to create test file: %v", err)
	}

	w, err := watcher.NewWatcher(testFile)
	if err != nil {
		jsonLogger.Fatalf("Failed to create watcher: %v", err)
	}

	w.OnChange = func(patch string) {
		jsonLogger.Infof("Patch generated: %s", patch)
	}

	errCh := make(chan error)
	err = w.Start(errCh)
	if err != nil {
		jsonLogger.Fatalf("Failed to start watcher: %v", err)
	}

	go func() {
		for err := range errCh {
			if err != nil {
				jsonLogger.Fatalf("Watcher error: %v", err)
			}
		}
	}()

	fmt.Printf("Watcher started for file: %s\n", testFile)
	fmt.Printf("Session ID: %s\n", w.SessionID)
	fmt.Printf("Agent ID: %s\n", w.AgentID)
	fmt.Println("\nNow make some edits to the file and watch the collaboration engine in action!")
	fmt.Println("   - Edit the file in another terminal or editor")
	fmt.Println("   - Watch for patch generation and session updates")
	fmt.Println("   - Check the .collab/ directory for persisted session data")
	fmt.Println("\nPress Ctrl+C to stop the watcher...")

	select {}
}

func cleanupTestFiles() {
	os.Remove("test-watcher.txt")
	os.RemoveAll(".collab")
}
