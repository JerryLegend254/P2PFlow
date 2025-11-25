package analytics

import (
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

// AccessType represents the type of file access
type AccessType string

const (
	AccessTypeRead   AccessType = "read"
	AccessTypeWrite  AccessType = "write"
	AccessTypeCreate AccessType = "create"
	AccessTypeDelete AccessType = "delete"
	AccessTypeSync   AccessType = "sync"
)

// AccessRecord represents a single file access event
type AccessRecord struct {
	FilePath   string     `json:"file_path"`
	AccessType AccessType `json:"access_type"`
	Timestamp  time.Time  `json:"timestamp"`
	PeerID     string     `json:"peer_id,omitempty"`
	SizeBytes  int64      `json:"size_bytes,omitempty"`
}

// FileStatistics holds detailed statistics for a specific file
type FileStatistics struct {
	FilePath         string               `json:"file_path"`
	TotalAccesses    int                  `json:"total_accesses"`
	ReadCount        int                  `json:"read_count"`
	WriteCount       int                  `json:"write_count"`
	CreateCount      int                  `json:"create_count"`
	DeleteCount      int                  `json:"delete_count"`
	SyncCount        int                  `json:"sync_count"`
	FirstAccess      time.Time            `json:"first_access"`
	LastAccess       time.Time            `json:"last_access"`
	TotalBytes       int64                `json:"total_bytes"`
	AccessHistory    []AccessRecord       `json:"access_history"`
	HourlyPattern    map[int]int          `json:"hourly_pattern"`      // Hour of day (0-23) -> count
	DayOfWeekPattern map[time.Weekday]int `json:"day_of_week_pattern"` // Day of week -> count
	PeerAccesses     map[string]int       `json:"peer_accesses"`       // Peer ID -> count
}

// AccessTracker tracks file access patterns
type AccessTracker struct {
	records     []AccessRecord
	fileStats   map[string]*FileStatistics
	storagePath string
	mu          sync.RWMutex
	startTime   time.Time
}

// NewAccessTracker creates a new access tracker
func NewAccessTracker(storagePath string) (*AccessTracker, error) {
	tracker := &AccessTracker{
		records:     make([]AccessRecord, 0),
		fileStats:   make(map[string]*FileStatistics),
		storagePath: storagePath,
		startTime:   time.Now(),
	}

	// Try to load existing data
	if err := tracker.Load(); err != nil {
		// If load fails, start fresh (file might not exist yet)
		// This is not an error for new installations
	}

	return tracker, nil
}

// RecordAccess records a file access event
func (t *AccessTracker) RecordAccess(filePath string, accessType AccessType) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	record := AccessRecord{
		FilePath:   filePath,
		AccessType: accessType,
		Timestamp:  now,
	}

	t.records = append(t.records, record)
	t.updateFileStats(record)
}

// RecordChange records a file change with size and peer info
func (t *AccessTracker) RecordChange(filePath string, sizeBytes int64, peerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	record := AccessRecord{
		FilePath:   filePath,
		AccessType: AccessTypeWrite,
		Timestamp:  now,
		PeerID:     peerID,
		SizeBytes:  sizeBytes,
	}

	t.records = append(t.records, record)
	t.updateFileStats(record)
}

// updateFileStats updates statistics for a file (must be called with lock held)
func (t *AccessTracker) updateFileStats(record AccessRecord) {
	stats, exists := t.fileStats[record.FilePath]
	if !exists {
		stats = &FileStatistics{
			FilePath:         record.FilePath,
			FirstAccess:      record.Timestamp,
			AccessHistory:    make([]AccessRecord, 0),
			HourlyPattern:    make(map[int]int),
			DayOfWeekPattern: make(map[time.Weekday]int),
			PeerAccesses:     make(map[string]int),
		}
		t.fileStats[record.FilePath] = stats
	}

	// Update counts
	stats.TotalAccesses++
	switch record.AccessType {
	case AccessTypeRead:
		stats.ReadCount++
	case AccessTypeWrite:
		stats.WriteCount++
	case AccessTypeCreate:
		stats.CreateCount++
	case AccessTypeDelete:
		stats.DeleteCount++
	case AccessTypeSync:
		stats.SyncCount++
	}

	// Update timestamps
	stats.LastAccess = record.Timestamp

	// Update bytes
	if record.SizeBytes > 0 {
		stats.TotalBytes += record.SizeBytes
	}

	// Update patterns
	hour := record.Timestamp.Hour()
	stats.HourlyPattern[hour]++

	dayOfWeek := record.Timestamp.Weekday()
	stats.DayOfWeekPattern[dayOfWeek]++

	// Update peer accesses
	if record.PeerID != "" {
		stats.PeerAccesses[record.PeerID]++
	}

	// Add to history (keep last 1000 records per file)
	stats.AccessHistory = append(stats.AccessHistory, record)
	if len(stats.AccessHistory) > 1000 {
		stats.AccessHistory = stats.AccessHistory[len(stats.AccessHistory)-1000:]
	}
}

// GetFileStatistics returns statistics for a specific file
func (t *AccessTracker) GetFileStatistics(filePath string) *FileStatistics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats, exists := t.fileStats[filePath]
	if !exists {
		return nil
	}

	// Return a copy to avoid race conditions
	statsCopy := *stats
	return &statsCopy
}

// GetStatistics returns overall statistics
func (t *AccessTracker) GetStatistics() *Statistics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := &Statistics{
		TotalAccesses:     len(t.records),
		UniqueFiles:       len(t.fileStats),
		MostAccessedFiles: make([]FileAccessSummary, 0),
		AccessesByHour:    make(map[int]int),
		AccessesByDay:     make(map[string]int),
		PeerActivity:      make(map[string]*PeerStatistics),
		StartTime:         t.startTime,
		LastUpdate:        time.Now(),
	}

	// Count changes and bytes
	peerStats := make(map[string]*PeerStatistics)

	for _, record := range t.records {
		if record.AccessType == AccessTypeWrite || record.AccessType == AccessTypeCreate {
			stats.TotalChanges++
			stats.TotalBytesChanged += record.SizeBytes

			// Update peer statistics
			if record.PeerID != "" {
				if _, exists := peerStats[record.PeerID]; !exists {
					peerStats[record.PeerID] = &PeerStatistics{
						PeerID: record.PeerID,
					}
				}
				peerStats[record.PeerID].ChangeCount++
				peerStats[record.PeerID].BytesSent += record.SizeBytes
				peerStats[record.PeerID].LastActivity = record.Timestamp
			}
		}

		// Update hourly pattern
		hour := record.Timestamp.Hour()
		stats.AccessesByHour[hour]++

		// Update daily pattern
		day := record.Timestamp.Format("2006-01-02")
		stats.AccessesByDay[day]++
	}

	stats.PeerActivity = peerStats

	// Get most accessed files
	type fileSummary struct {
		path  string
		stats *FileStatistics
	}

	files := make([]fileSummary, 0, len(t.fileStats))
	for path, fileStats := range t.fileStats {
		files = append(files, fileSummary{path: path, stats: fileStats})
	}

	// Sort by access count
	sort.Slice(files, func(i, j int) bool {
		return files[i].stats.TotalAccesses > files[j].stats.TotalAccesses
	})

	// Take top 10
	limit := 10
	if len(files) < limit {
		limit = len(files)
	}

	for i := 0; i < limit; i++ {
		f := files[i]

		// Calculate average access frequency
		daysSinceFirst := time.Since(f.stats.FirstAccess).Hours() / 24
		if daysSinceFirst < 1 {
			daysSinceFirst = 1 // Avoid division by zero
		}
		avgFreq := float64(f.stats.TotalAccesses) / daysSinceFirst

		stats.MostAccessedFiles = append(stats.MostAccessedFiles, FileAccessSummary{
			FilePath:      f.path,
			AccessCount:   f.stats.TotalAccesses,
			ChangeCount:   f.stats.WriteCount + f.stats.CreateCount,
			LastAccess:    f.stats.LastAccess,
			TotalBytes:    f.stats.TotalBytes,
			AvgAccessFreq: avgFreq,
		})
	}

	return stats
}

// GetRecentAccesses returns the most recent access records
func (t *AccessTracker) GetRecentAccesses(limit int) []AccessRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit > len(t.records) {
		limit = len(t.records)
	}

	// Return last N records
	result := make([]AccessRecord, limit)
	copy(result, t.records[len(t.records)-limit:])
	return result
}

// GetAccessesSince returns all accesses since a given time
func (t *AccessTracker) GetAccessesSince(since time.Time) []AccessRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]AccessRecord, 0)
	for _, record := range t.records {
		if record.Timestamp.After(since) {
			result = append(result, record)
		}
	}
	return result
}

// GetCoAccessedFiles returns files that are often accessed together
func (t *AccessTracker) GetCoAccessedFiles(filePath string, limit int) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Find all accesses of the target file
	targetAccesses := make([]time.Time, 0)
	for _, record := range t.records {
		if record.FilePath == filePath {
			targetAccesses = append(targetAccesses, record.Timestamp)
		}
	}

	// Count files accessed within 5 minutes of target file
	coAccessCounts := make(map[string]int)
	timeWindow := 5 * time.Minute

	for _, targetTime := range targetAccesses {
		for _, record := range t.records {
			if record.FilePath == filePath {
				continue // Skip the target file itself
			}

			timeDiff := record.Timestamp.Sub(targetTime)
			if timeDiff >= -timeWindow && timeDiff <= timeWindow {
				coAccessCounts[record.FilePath]++
			}
		}
	}

	// Sort by count
	type fileCount struct {
		path  string
		count int
	}

	files := make([]fileCount, 0, len(coAccessCounts))
	for path, count := range coAccessCounts {
		files = append(files, fileCount{path: path, count: count})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].count > files[j].count
	})

	// Return top N
	if limit > len(files) {
		limit = len(files)
	}

	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = files[i].path
	}

	return result
}

// Save persists the tracker data to disk
func (t *AccessTracker) Save() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	data := struct {
		Records   []AccessRecord             `json:"records"`
		FileStats map[string]*FileStatistics `json:"file_stats"`
		StartTime time.Time                  `json:"start_time"`
	}{
		Records:   t.records,
		FileStats: t.fileStats,
		StartTime: t.startTime,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(t.storagePath, jsonData, 0644)
}

// Load loads tracker data from disk
func (t *AccessTracker) Load() error {
	data, err := os.ReadFile(t.storagePath)
	if err != nil {
		return err
	}

	var loaded struct {
		Records   []AccessRecord             `json:"records"`
		FileStats map[string]*FileStatistics `json:"file_stats"`
		StartTime time.Time                  `json:"start_time"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.records = loaded.Records
	t.fileStats = loaded.FileStats
	t.startTime = loaded.StartTime

	return nil
}

// CleanupOldData removes records older than the specified duration
func (t *AccessTracker) CleanupOldData(maxAge time.Duration) error {
	t.mu.Lock()

	cutoff := time.Now().Add(-maxAge)

	// Filter records
	newRecords := make([]AccessRecord, 0)
	for _, record := range t.records {
		if record.Timestamp.After(cutoff) {
			newRecords = append(newRecords, record)
		}
	}

	t.records = newRecords

	// Rebuild file stats from remaining records
	t.fileStats = make(map[string]*FileStatistics)
	for _, record := range t.records {
		t.updateFileStats(record)
	}

	// Prepare data for saving while holding the lock
	data := struct {
		Records   []AccessRecord             `json:"records"`
		FileStats map[string]*FileStatistics `json:"file_stats"`
		StartTime time.Time                  `json:"start_time"`
	}{
		Records:   t.records,
		FileStats: t.fileStats,
		StartTime: t.startTime,
	}

	// Release lock before file I/O
	t.mu.Unlock()

	// Save to disk without holding the lock
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(t.storagePath, jsonData, 0644)
}
