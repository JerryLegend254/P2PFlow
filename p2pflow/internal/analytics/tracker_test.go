package analytics

import (
	"os"
	"testing"
	"time"
)

func TestAccessTracker_RecordAccess(t *testing.T) {
	// Create temporary file for testing
	tmpFile, err := os.CreateTemp("", "tracker_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create tracker
	tracker, err := NewAccessTracker(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}

	// Record some accesses
	tracker.RecordAccess("file1.go", AccessTypeWrite)
	tracker.RecordAccess("file1.go", AccessTypeRead)
	tracker.RecordAccess("file2.go", AccessTypeCreate)

	// Check statistics
	stats := tracker.GetStatistics()
	if stats.TotalAccesses != 3 {
		t.Errorf("Expected 3 total accesses, got %d", stats.TotalAccesses)
	}

	if stats.UniqueFiles != 2 {
		t.Errorf("Expected 2 unique files, got %d", stats.UniqueFiles)
	}

	// Check file statistics
	file1Stats := tracker.GetFileStatistics("file1.go")
	if file1Stats == nil {
		t.Fatal("Expected file1.go statistics to exist")
	}

	if file1Stats.TotalAccesses != 2 {
		t.Errorf("Expected 2 accesses for file1.go, got %d", file1Stats.TotalAccesses)
	}

	if file1Stats.WriteCount != 1 {
		t.Errorf("Expected 1 write for file1.go, got %d", file1Stats.WriteCount)
	}

	if file1Stats.ReadCount != 1 {
		t.Errorf("Expected 1 read for file1.go, got %d", file1Stats.ReadCount)
	}
}

func TestAccessTracker_SaveAndLoad(t *testing.T) {
	// Create temporary file for testing
	tmpFile, err := os.CreateTemp("", "tracker_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create tracker and record some data
	tracker1, err := NewAccessTracker(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}

	tracker1.RecordAccess("file1.go", AccessTypeWrite)
	tracker1.RecordAccess("file2.go", AccessTypeRead)

	// Save
	if err := tracker1.Save(); err != nil {
		t.Fatalf("Failed to save tracker: %v", err)
	}

	// Create new tracker and load
	tracker2, err := NewAccessTracker(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create second tracker: %v", err)
	}

	// Check loaded data
	stats := tracker2.GetStatistics()
	if stats.TotalAccesses != 2 {
		t.Errorf("Expected 2 total accesses after load, got %d", stats.TotalAccesses)
	}

	if stats.UniqueFiles != 2 {
		t.Errorf("Expected 2 unique files after load, got %d", stats.UniqueFiles)
	}
}

func TestAccessTracker_GetCoAccessedFiles(t *testing.T) {
	// Create temporary file for testing
	tmpFile, err := os.CreateTemp("", "tracker_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tracker, err := NewAccessTracker(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}

	// Record co-accessed files (within time window)
	now := time.Now()
	tracker.records = []AccessRecord{
		{FilePath: "file1.go", Timestamp: now},
		{FilePath: "file2.go", Timestamp: now.Add(1 * time.Minute)},
		{FilePath: "file3.go", Timestamp: now.Add(2 * time.Minute)},
		{FilePath: "file1.go", Timestamp: now.Add(10 * time.Minute)},
		{FilePath: "file2.go", Timestamp: now.Add(11 * time.Minute)},
	}

	// Get co-accessed files for file1.go
	coAccessed := tracker.GetCoAccessedFiles("file1.go", 5)

	// file2.go should be the most co-accessed (accessed twice near file1.go)
	if len(coAccessed) == 0 {
		t.Fatal("Expected at least one co-accessed file")
	}

	if coAccessed[0] != "file2.go" {
		t.Errorf("Expected file2.go as most co-accessed, got %s", coAccessed[0])
	}
}

func TestAccessTracker_CleanupOldData(t *testing.T) {
	// Create temporary file for testing
	tmpFile, err := os.CreateTemp("", "tracker_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tracker, err := NewAccessTracker(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create tracker: %v", err)
	}

	// Record old and new data
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	newTime := time.Now()

	tracker.records = []AccessRecord{
		{FilePath: "old_file.go", Timestamp: oldTime, AccessType: AccessTypeWrite},
		{FilePath: "new_file.go", Timestamp: newTime, AccessType: AccessTypeWrite},
	}

	// Rebuild file stats
	tracker.fileStats = make(map[string]*FileStatistics)
	for _, record := range tracker.records {
		tracker.updateFileStats(record)
	}

	// Cleanup data older than 5 days
	if err := tracker.CleanupOldData(5 * 24 * time.Hour); err != nil {
		t.Fatalf("Failed to cleanup old data: %v", err)
	}

	// Check that only new data remains
	stats := tracker.GetStatistics()
	if stats.TotalAccesses != 1 {
		t.Errorf("Expected 1 access after cleanup, got %d", stats.TotalAccesses)
	}

	if stats.UniqueFiles != 1 {
		t.Errorf("Expected 1 unique file after cleanup, got %d", stats.UniqueFiles)
	}

	// Old file should be gone
	oldFileStats := tracker.GetFileStatistics("old_file.go")
	if oldFileStats != nil {
		t.Error("Expected old_file.go to be cleaned up")
	}

	// New file should remain
	newFileStats := tracker.GetFileStatistics("new_file.go")
	if newFileStats == nil {
		t.Error("Expected new_file.go to remain")
	}
}
