package analytics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AnalyticsEngine is the main orchestrator for all analytics functionality
type AnalyticsEngine struct {
	tracker      *AccessTracker
	predictor    *Predictor
	prefetch     *PrefetchEngine
	anomaly      *AnomalyDetector
	bandwidth    *BandwidthAllocator
	storagePath  string
	mu           sync.RWMutex
	enabled      bool
}

// Config holds configuration for the analytics engine
type Config struct {
	Enabled           bool
	StoragePath       string
	PrefetchEnabled   bool
	AnomalyDetection  bool
	MaxHistoryDays    int // How many days of history to keep
	MinConfidence     float64 // Minimum confidence for prefetch suggestions
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:          true,
		StoragePath:      ".collab/analytics",
		PrefetchEnabled:  true,
		AnomalyDetection: true,
		MaxHistoryDays:   30,
		MinConfidence:    0.6,
	}
}

// NewAnalyticsEngine creates a new analytics engine
func NewAnalyticsEngine(config *Config) (*AnalyticsEngine, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create storage directory
	if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create analytics storage directory: %w", err)
	}

	engine := &AnalyticsEngine{
		storagePath: config.StoragePath,
		enabled:     config.Enabled,
	}

	// Initialize components
	var err error

	engine.tracker, err = NewAccessTracker(filepath.Join(config.StoragePath, "access_log.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to create access tracker: %w", err)
	}

	engine.predictor = NewPredictor(engine.tracker, config.MinConfidence)
	engine.prefetch = NewPrefetchEngine(engine.predictor, config.PrefetchEnabled)
	engine.anomaly = NewAnomalyDetector(engine.tracker)
	engine.bandwidth = NewBandwidthAllocator(engine.tracker)

	return engine, nil
}

// RecordFileAccess records when a file is accessed
func (e *AnalyticsEngine) RecordFileAccess(filePath string, accessType AccessType) {
	if !e.enabled {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.tracker.RecordAccess(filePath, accessType)
}

// RecordFileChange records when a file is changed
func (e *AnalyticsEngine) RecordFileChange(filePath string, sizeBytes int64, peerID string) {
	if !e.enabled {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.tracker.RecordChange(filePath, sizeBytes, peerID)
}

// GetPrefetchSuggestions returns files that should be prefetched
func (e *AnalyticsEngine) GetPrefetchSuggestions(currentContext []string, maxSuggestions int) []PrefetchSuggestion {
	if !e.enabled {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.prefetch.GetSuggestions(currentContext, maxSuggestions)
}

// GetFileImportance returns an importance score for a file (0.0 to 1.0)
func (e *AnalyticsEngine) GetFileImportance(filePath string) float64 {
	if !e.enabled {
		return 0.5 // Default medium priority
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.bandwidth.GetFileImportance(filePath)
}

// DetectAnomalies checks for unusual patterns
func (e *AnalyticsEngine) DetectAnomalies() []Anomaly {
	if !e.enabled {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.anomaly.Detect()
}

// GetStatistics returns overall statistics
func (e *AnalyticsEngine) GetStatistics() *Statistics {
	if !e.enabled {
		return &Statistics{}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.tracker.GetStatistics()
}

// GetFileStatistics returns statistics for a specific file
func (e *AnalyticsEngine) GetFileStatistics(filePath string) *FileStatistics {
	if !e.enabled {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.tracker.GetFileStatistics(filePath)
}

// PredictNextFiles predicts which files are likely to be accessed next
func (e *AnalyticsEngine) PredictNextFiles(currentFile string, limit int) []Prediction {
	if !e.enabled {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.predictor.PredictNext(currentFile, limit)
}

// Save persists all analytics data to disk
func (e *AnalyticsEngine) Save() error {
	if !e.enabled {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.tracker.Save()
}

// Load loads analytics data from disk
func (e *AnalyticsEngine) Load() error {
	if !e.enabled {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return e.tracker.Load()
}

// Close saves data and performs cleanup
func (e *AnalyticsEngine) Close() error {
	if !e.enabled {
		return nil
	}

	return e.Save()
}

// CleanupOldData removes data older than the configured retention period
func (e *AnalyticsEngine) CleanupOldData(maxAge time.Duration) error {
	if !e.enabled {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	return e.tracker.CleanupOldData(maxAge)
}

// Statistics holds overall analytics statistics
type Statistics struct {
	TotalAccesses     int                        `json:"total_accesses"`
	TotalChanges      int                        `json:"total_changes"`
	UniqueFiles       int                        `json:"unique_files"`
	TotalBytesChanged int64                      `json:"total_bytes_changed"`
	MostAccessedFiles []FileAccessSummary        `json:"most_accessed_files"`
	AccessesByHour    map[int]int                `json:"accesses_by_hour"`
	AccessesByDay     map[string]int             `json:"accesses_by_day"`
	PeerActivity      map[string]*PeerStatistics `json:"peer_activity"`
	StartTime         time.Time                  `json:"start_time"`
	LastUpdate        time.Time                  `json:"last_update"`
}

// FileAccessSummary summarizes access patterns for a file
type FileAccessSummary struct {
	FilePath      string    `json:"file_path"`
	AccessCount   int       `json:"access_count"`
	ChangeCount   int       `json:"change_count"`
	LastAccess    time.Time `json:"last_access"`
	TotalBytes    int64     `json:"total_bytes"`
	AvgAccessFreq float64   `json:"avg_access_freq"` // Accesses per day
}

// PeerStatistics holds statistics about peer activity
type PeerStatistics struct {
	PeerID       string    `json:"peer_id"`
	ChangeCount  int       `json:"change_count"`
	BytesSent    int64     `json:"bytes_sent"`
	LastActivity time.Time `json:"last_activity"`
}

// ToJSON converts statistics to JSON
func (s *Statistics) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
