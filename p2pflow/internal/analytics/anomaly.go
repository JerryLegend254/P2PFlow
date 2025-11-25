package analytics

import (
	"math"
	"time"
)

// AnomalyType represents the type of anomaly detected
type AnomalyType string

const (
	AnomalyTypeUnusualAccessPattern AnomalyType = "unusual_access_pattern"
	AnomalyTypeUnusualTime          AnomalyType = "unusual_time"
	AnomalyTypeSuspiciousFrequency  AnomalyType = "suspicious_frequency"
	AnomalyTypeUnexpectedPeer       AnomalyType = "unexpected_peer"
	AnomalyTypeLargeFileChange      AnomalyType = "large_file_change"
	AnomalyTypeRapidChanges         AnomalyType = "rapid_changes"
	AnomalyTypeUnusualFileSize      AnomalyType = "unusual_file_size"
)

// AnomalySeverity represents how severe an anomaly is
type AnomalySeverity string

const (
	SeverityLow      AnomalySeverity = "low"
	SeverityMedium   AnomalySeverity = "medium"
	SeverityHigh     AnomalySeverity = "high"
	SeverityCritical AnomalySeverity = "critical"
)

// Anomaly represents a detected anomaly
type Anomaly struct {
	Type        AnomalyType            `json:"type"`
	Severity    AnomalySeverity        `json:"severity"`
	FilePath    string                 `json:"file_path,omitempty"`
	PeerID      string                 `json:"peer_id,omitempty"`
	Description string                 `json:"description"`
	Score       float64                `json:"score"` // How anomalous (0-1, higher = more anomalous)
	Timestamp   time.Time              `json:"timestamp"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// AnomalyDetector detects unusual patterns in file access and synchronization
type AnomalyDetector struct {
	tracker *AccessTracker
}

// NewAnomalyDetector creates a new anomaly detector
func NewAnomalyDetector(tracker *AccessTracker) *AnomalyDetector {
	return &AnomalyDetector{
		tracker: tracker,
	}
}

// Detect runs all anomaly detection algorithms and returns detected anomalies
func (d *AnomalyDetector) Detect() []Anomaly {
	anomalies := make([]Anomaly, 0)

	// Run different detection strategies
	anomalies = append(anomalies, d.detectUnusualAccessTimes()...)
	anomalies = append(anomalies, d.detectRapidChanges()...)
	anomalies = append(anomalies, d.detectUnusualFrequency()...)
	anomalies = append(anomalies, d.detectLargeFileChanges()...)
	anomalies = append(anomalies, d.detectUnusualPeerActivity()...)

	return anomalies
}

// detectUnusualAccessTimes detects files accessed at unusual times
func (d *AnomalyDetector) detectUnusualAccessTimes() []Anomaly {
	anomalies := make([]Anomaly, 0)

	// Get recent accesses (last 24 hours)
	recentAccesses := d.tracker.GetAccessesSince(time.Now().Add(-24 * time.Hour))

	for _, record := range recentAccesses {
		fileStats := d.tracker.GetFileStatistics(record.FilePath)
		if fileStats == nil || fileStats.TotalAccesses < 10 {
			continue // Need sufficient history
		}

		// Check if access time is unusual for this file
		hour := record.Timestamp.Hour()
		hourCount := fileStats.HourlyPattern[hour]
		totalAccesses := fileStats.TotalAccesses

		// Calculate expected probability for this hour
		expectedProb := float64(hourCount) / float64(totalAccesses)

		// If this hour accounts for < 5% of historical accesses, it's unusual
		if expectedProb < 0.05 && totalAccesses > 20 {
			score := 1.0 - (expectedProb * 10) // Score 0.5 to 1.0
			severity := d.calculateSeverity(score)

			anomalies = append(anomalies, Anomaly{
				Type:        AnomalyTypeUnusualTime,
				Severity:    severity,
				FilePath:    record.FilePath,
				Description: "File accessed at unusual time",
				Score:       score,
				Timestamp:   record.Timestamp,
				Details: map[string]interface{}{
					"hour":          hour,
					"expected_prob": expectedProb,
				},
			})
		}
	}

	return anomalies
}

// detectRapidChanges detects files being changed very rapidly
func (d *AnomalyDetector) detectRapidChanges() []Anomaly {
	anomalies := make([]Anomaly, 0)

	stats := d.tracker.GetStatistics()

	for _, summary := range stats.MostAccessedFiles {
		fileStats := d.tracker.GetFileStatistics(summary.FilePath)
		if fileStats == nil {
			continue
		}

		// Count changes in last hour
		oneHourAgo := time.Now().Add(-1 * time.Hour)
		recentChanges := 0

		for _, record := range fileStats.AccessHistory {
			if record.Timestamp.After(oneHourAgo) &&
				(record.AccessType == AccessTypeWrite || record.AccessType == AccessTypeCreate) {
				recentChanges++
			}
		}

		// If more than 10 changes in an hour, it's potentially suspicious
		if recentChanges > 10 {
			score := math.Min(float64(recentChanges)/50.0, 1.0)
			severity := d.calculateSeverity(score)

			anomalies = append(anomalies, Anomaly{
				Type:        AnomalyTypeRapidChanges,
				Severity:    severity,
				FilePath:    summary.FilePath,
				Description: "File changing very rapidly",
				Score:       score,
				Timestamp:   time.Now(),
				Details: map[string]interface{}{
					"changes_last_hour": recentChanges,
				},
			})
		}
	}

	return anomalies
}

// detectUnusualFrequency detects unusual changes in access frequency
func (d *AnomalyDetector) detectUnusualFrequency() []Anomaly {
	anomalies := make([]Anomaly, 0)

	stats := d.tracker.GetStatistics()

	for _, summary := range stats.MostAccessedFiles {
		fileStats := d.tracker.GetFileStatistics(summary.FilePath)
		if fileStats == nil || fileStats.TotalAccesses < 20 {
			continue // Need sufficient history
		}

		// Calculate historical average access frequency
		daysSinceFirst := time.Since(fileStats.FirstAccess).Hours() / 24
		if daysSinceFirst < 1 {
			daysSinceFirst = 1
		}
		avgAccessesPerDay := float64(fileStats.TotalAccesses) / daysSinceFirst

		// Count accesses in last 24 hours
		recentAccesses := 0
		oneDayAgo := time.Now().Add(-24 * time.Hour)
		for _, record := range fileStats.AccessHistory {
			if record.Timestamp.After(oneDayAgo) {
				recentAccesses++
			}
		}

		// Check if recent frequency is significantly different from average
		// Using 3x as threshold (3 standard deviations equivalent)
		if float64(recentAccesses) > avgAccessesPerDay*3 {
			// Spike in activity
			score := math.Min(float64(recentAccesses)/(avgAccessesPerDay*3), 1.0)
			severity := d.calculateSeverity(score)

			anomalies = append(anomalies, Anomaly{
				Type:        AnomalyTypeSuspiciousFrequency,
				Severity:    severity,
				FilePath:    summary.FilePath,
				Description: "Unusual spike in access frequency",
				Score:       score,
				Timestamp:   time.Now(),
				Details: map[string]interface{}{
					"avg_accesses_per_day": avgAccessesPerDay,
					"recent_accesses":      recentAccesses,
				},
			})
		}
	}

	return anomalies
}

// detectLargeFileChanges detects unusually large file changes
func (d *AnomalyDetector) detectLargeFileChanges() []Anomaly {
	anomalies := make([]Anomaly, 0)

	// Get recent changes
	recentAccesses := d.tracker.GetAccessesSince(time.Now().Add(-24 * time.Hour))

	for _, record := range recentAccesses {
		if record.AccessType != AccessTypeWrite && record.AccessType != AccessTypeCreate {
			continue
		}

		fileStats := d.tracker.GetFileStatistics(record.FilePath)
		if fileStats == nil || fileStats.WriteCount < 5 {
			continue // Need some history
		}

		// Calculate average change size
		avgSize := fileStats.TotalBytes / int64(fileStats.WriteCount)

		// If this change is > 10x average, it's unusual
		if record.SizeBytes > 0 && record.SizeBytes > avgSize*10 {
			score := math.Min(float64(record.SizeBytes)/float64(avgSize*10), 1.0)
			severity := d.calculateSeverity(score)

			anomalies = append(anomalies, Anomaly{
				Type:        AnomalyTypeLargeFileChange,
				Severity:    severity,
				FilePath:    record.FilePath,
				Description: "Unusually large file change",
				Score:       score,
				Timestamp:   record.Timestamp,
				Details: map[string]interface{}{
					"change_size_bytes": record.SizeBytes,
					"avg_size_bytes":    avgSize,
				},
			})
		}
	}

	return anomalies
}

// detectUnusualPeerActivity detects unusual patterns in peer activity
func (d *AnomalyDetector) detectUnusualPeerActivity() []Anomaly {
	anomalies := make([]Anomaly, 0)

	stats := d.tracker.GetStatistics()

	// Check for peers with unusual activity levels
	if len(stats.PeerActivity) < 2 {
		return anomalies // Need multiple peers to compare
	}

	// Calculate average peer activity
	totalChanges := 0
	for _, peerStats := range stats.PeerActivity {
		totalChanges += peerStats.ChangeCount
	}
	avgChangesPerPeer := float64(totalChanges) / float64(len(stats.PeerActivity))

	// Check each peer
	for peerID, peerStats := range stats.PeerActivity {
		// If a peer has > 5x average activity, it might be suspicious
		if float64(peerStats.ChangeCount) > avgChangesPerPeer*5 {
			score := math.Min(float64(peerStats.ChangeCount)/(avgChangesPerPeer*5), 1.0)
			severity := d.calculateSeverity(score)

			anomalies = append(anomalies, Anomaly{
				Type:        AnomalyTypeUnexpectedPeer,
				Severity:    severity,
				PeerID:      peerID,
				Description: "Peer has unusually high activity",
				Score:       score,
				Timestamp:   time.Now(),
				Details: map[string]interface{}{
					"peer_changes":     peerStats.ChangeCount,
					"avg_peer_changes": avgChangesPerPeer,
				},
			})
		}
	}

	return anomalies
}

// calculateSeverity calculates severity based on anomaly score
func (d *AnomalyDetector) calculateSeverity(score float64) AnomalySeverity {
	if score >= 0.9 {
		return SeverityCritical
	} else if score >= 0.7 {
		return SeverityHigh
	} else if score >= 0.5 {
		return SeverityMedium
	}
	return SeverityLow
}

// DetectForFile checks for anomalies related to a specific file
func (d *AnomalyDetector) DetectForFile(filePath string) []Anomaly {
	allAnomalies := d.Detect()
	fileAnomalies := make([]Anomaly, 0)

	for _, anomaly := range allAnomalies {
		if anomaly.FilePath == filePath {
			fileAnomalies = append(fileAnomalies, anomaly)
		}
	}

	return fileAnomalies
}

// DetectForPeer checks for anomalies related to a specific peer
func (d *AnomalyDetector) DetectForPeer(peerID string) []Anomaly {
	allAnomalies := d.Detect()
	peerAnomalies := make([]Anomaly, 0)

	for _, anomaly := range allAnomalies {
		if anomaly.PeerID == peerID {
			peerAnomalies = append(peerAnomalies, anomaly)
		}
	}

	return peerAnomalies
}

// GetHighSeverityAnomalies returns only high and critical severity anomalies
func (d *AnomalyDetector) GetHighSeverityAnomalies() []Anomaly {
	allAnomalies := d.Detect()
	highSeverity := make([]Anomaly, 0)

	for _, anomaly := range allAnomalies {
		if anomaly.Severity == SeverityHigh || anomaly.Severity == SeverityCritical {
			highSeverity = append(highSeverity, anomaly)
		}
	}

	return highSeverity
}
