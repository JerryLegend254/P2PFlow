package analytics

import (
	"math"
	"sort"
	"time"
)

// Prediction represents a predicted file access
type Prediction struct {
	FilePath   string    `json:"file_path"`
	Confidence float64   `json:"confidence"` // 0.0 to 1.0
	Reason     string    `json:"reason"`     // Why this prediction was made
	Score      float64   `json:"score"`      // Internal score for ranking
	Timestamp  time.Time `json:"timestamp"`
}

// Predictor predicts future file accesses based on historical patterns
type Predictor struct {
	tracker       *AccessTracker
	minConfidence float64
}

// NewPredictor creates a new predictor
func NewPredictor(tracker *AccessTracker, minConfidence float64) *Predictor {
	return &Predictor{
		tracker:       tracker,
		minConfidence: minConfidence,
	}
}

// PredictNext predicts which files are likely to be accessed next
func (p *Predictor) PredictNext(currentFile string, limit int) []Prediction {
	predictions := make([]Prediction, 0)

	// Strategy 1: Co-access patterns
	coAccessPredictions := p.predictByCoAccess(currentFile)
	predictions = append(predictions, coAccessPredictions...)

	// Strategy 2: Time-based patterns
	timePredictions := p.predictByTimePattern()
	predictions = append(predictions, timePredictions...)

	// Strategy 3: Frequency-based predictions
	freqPredictions := p.predictByFrequency()
	predictions = append(predictions, freqPredictions...)

	// Strategy 4: Recent trend predictions
	trendPredictions := p.predictByRecentTrend()
	predictions = append(predictions, trendPredictions...)

	// Merge and deduplicate predictions
	merged := p.mergePredictions(predictions)

	// Filter by minimum confidence
	filtered := make([]Prediction, 0)
	for _, pred := range merged {
		if pred.Confidence >= p.minConfidence {
			filtered = append(filtered, pred)
		}
	}

	// Sort by confidence (descending)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Confidence > filtered[j].Confidence
	})

	// Return top N
	if limit > len(filtered) {
		limit = len(filtered)
	}

	return filtered[:limit]
}

// predictByCoAccess predicts based on files frequently accessed together
func (p *Predictor) predictByCoAccess(currentFile string) []Prediction {
	coAccessedFiles := p.tracker.GetCoAccessedFiles(currentFile, 10)

	predictions := make([]Prediction, 0, len(coAccessedFiles))
	for i, filePath := range coAccessedFiles {
		// Higher confidence for more frequently co-accessed files
		confidence := 0.9 - (float64(i) * 0.1)
		if confidence < 0.5 {
			confidence = 0.5
		}

		predictions = append(predictions, Prediction{
			FilePath:   filePath,
			Confidence: confidence,
			Reason:     "Frequently accessed together with " + currentFile,
			Score:      confidence,
			Timestamp:  time.Now(),
		})
	}

	return predictions
}

// predictByTimePattern predicts based on time-of-day patterns
func (p *Predictor) predictByTimePattern() []Prediction {
	stats := p.tracker.GetStatistics()
	now := time.Now()
	currentHour := now.Hour()

	predictions := make([]Prediction, 0)

	// Find files with strong hourly patterns matching current hour
	for _, summary := range stats.MostAccessedFiles {
		fileStats := p.tracker.GetFileStatistics(summary.FilePath)
		if fileStats == nil {
			continue
		}

		// Check if this file is frequently accessed at this hour
		currentHourCount := fileStats.HourlyPattern[currentHour]
		totalAccesses := fileStats.TotalAccesses

		if totalAccesses == 0 {
			continue
		}

		// Calculate the proportion of accesses at this hour
		hourProportion := float64(currentHourCount) / float64(totalAccesses)

		// If > 20% of accesses happen at this hour, it's a strong pattern
		if hourProportion > 0.2 {
			confidence := math.Min(hourProportion*2, 0.95)

			predictions = append(predictions, Prediction{
				FilePath:   summary.FilePath,
				Confidence: confidence,
				Reason:     "Typically accessed at this time of day",
				Score:      confidence,
				Timestamp:  time.Now(),
			})
		}
	}

	return predictions
}

// predictByFrequency predicts highly frequently accessed files
func (p *Predictor) predictByFrequency() []Prediction {
	stats := p.tracker.GetStatistics()
	predictions := make([]Prediction, 0)

	// Consider top frequently accessed files
	limit := 5
	if len(stats.MostAccessedFiles) < limit {
		limit = len(stats.MostAccessedFiles)
	}

	for i := 0; i < limit; i++ {
		summary := stats.MostAccessedFiles[i]

		// Check if file was accessed recently
		timeSinceLastAccess := time.Since(summary.LastAccess)

		// Higher confidence for more frequently accessed files
		// Lower confidence if not accessed recently
		baseConfidence := 0.7 - (float64(i) * 0.1)

		// Decay confidence based on time since last access
		daysSinceAccess := timeSinceLastAccess.Hours() / 24
		decayFactor := math.Exp(-daysSinceAccess / 7) // Exponential decay with 7-day half-life

		confidence := baseConfidence * decayFactor

		if confidence >= 0.3 {
			predictions = append(predictions, Prediction{
				FilePath:   summary.FilePath,
				Confidence: confidence,
				Reason:     "Frequently accessed file",
				Score:      confidence,
				Timestamp:  time.Now(),
			})
		}
	}

	return predictions
}

// predictByRecentTrend predicts based on recent access trends
func (p *Predictor) predictByRecentTrend() []Prediction {
	// Get recent accesses (last 24 hours)
	recentAccesses := p.tracker.GetAccessesSince(time.Now().Add(-24 * time.Hour))

	// Count file accesses in recent period
	fileCounts := make(map[string]int)
	for _, record := range recentAccesses {
		fileCounts[record.FilePath]++
	}

	// Sort by recent access count
	type fileCount struct {
		path  string
		count int
	}

	files := make([]fileCount, 0, len(fileCounts))
	for path, count := range fileCounts {
		files = append(files, fileCount{path: path, count: count})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].count > files[j].count
	})

	predictions := make([]Prediction, 0)

	// Take top 5 recently trending files
	limit := 5
	if len(files) < limit {
		limit = len(files)
	}

	for i := 0; i < limit; i++ {
		f := files[i]

		// Calculate confidence based on recent access frequency
		accessesPerHour := float64(f.count) / 24.0
		confidence := math.Min(accessesPerHour/5.0, 0.85) // Cap at 0.85

		if confidence >= 0.3 {
			predictions = append(predictions, Prediction{
				FilePath:   f.path,
				Confidence: confidence,
				Reason:     "Recently trending",
				Score:      confidence,
				Timestamp:  time.Now(),
			})
		}
	}

	return predictions
}

// mergePredictions combines multiple predictions for the same file
func (p *Predictor) mergePredictions(predictions []Prediction) []Prediction {
	// Group by file path
	fileMap := make(map[string][]Prediction)
	for _, pred := range predictions {
		fileMap[pred.FilePath] = append(fileMap[pred.FilePath], pred)
	}

	// Merge predictions for each file
	merged := make([]Prediction, 0, len(fileMap))
	for filePath, preds := range fileMap {
		if len(preds) == 1 {
			merged = append(merged, preds[0])
			continue
		}

		// Combine confidences using weighted average
		// Multiple signals increase confidence
		totalConfidence := 0.0
		reasons := make([]string, 0)
		maxScore := 0.0

		for _, pred := range preds {
			totalConfidence += pred.Confidence
			reasons = append(reasons, pred.Reason)
			if pred.Score > maxScore {
				maxScore = pred.Score
			}
		}

		// Average confidence, but boost if multiple signals agree
		avgConfidence := totalConfidence / float64(len(preds))
		boost := math.Min(float64(len(preds))*0.1, 0.2)
		finalConfidence := math.Min(avgConfidence+boost, 0.99)

		// Combine reasons
		combinedReason := reasons[0]
		if len(reasons) > 1 {
			combinedReason = "Multiple signals: " + reasons[0]
			for i := 1; i < len(reasons) && i < 3; i++ {
				combinedReason += ", " + reasons[i]
			}
		}

		merged = append(merged, Prediction{
			FilePath:   filePath,
			Confidence: finalConfidence,
			Reason:     combinedReason,
			Score:      maxScore,
			Timestamp:  time.Now(),
		})
	}

	return merged
}

// PredictByDayOfWeek predicts files likely to be accessed on a specific day
func (p *Predictor) PredictByDayOfWeek(dayOfWeek time.Weekday, limit int) []Prediction {
	stats := p.tracker.GetStatistics()
	predictions := make([]Prediction, 0)

	for _, summary := range stats.MostAccessedFiles {
		fileStats := p.tracker.GetFileStatistics(summary.FilePath)
		if fileStats == nil {
			continue
		}

		// Check pattern for this day of week
		dayCount := fileStats.DayOfWeekPattern[dayOfWeek]
		totalAccesses := fileStats.TotalAccesses

		if totalAccesses == 0 {
			continue
		}

		// Calculate proportion
		dayProportion := float64(dayCount) / float64(totalAccesses)

		// If > 15% of accesses happen on this day, it's a pattern
		if dayProportion > 0.15 {
			confidence := math.Min(dayProportion*3, 0.9)

			predictions = append(predictions, Prediction{
				FilePath:   summary.FilePath,
				Confidence: confidence,
				Reason:     "Frequently accessed on " + dayOfWeek.String() + "s",
				Score:      confidence,
				Timestamp:  time.Now(),
			})
		}
	}

	// Sort by confidence
	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].Confidence > predictions[j].Confidence
	})

	if limit > len(predictions) {
		limit = len(predictions)
	}

	return predictions[:limit]
}

// CalculateAccessProbability calculates the probability a file will be accessed in the next N hours
func (p *Predictor) CalculateAccessProbability(filePath string, hoursAhead int) float64 {
	fileStats := p.tracker.GetFileStatistics(filePath)
	if fileStats == nil {
		return 0.0
	}

	// Calculate average time between accesses
	if fileStats.TotalAccesses < 2 {
		return 0.1 // Not enough data
	}

	timeSinceFirst := time.Since(fileStats.FirstAccess)
	avgTimeBetweenAccesses := timeSinceFirst / time.Duration(fileStats.TotalAccesses)

	// Calculate how long since last access
	timeSinceLastAccess := time.Since(fileStats.LastAccess)

	// If time since last access is close to average interval, probability is higher
	ratio := timeSinceLastAccess.Hours() / avgTimeBetweenAccesses.Hours()

	// Use a sigmoid function to calculate probability
	// Probability increases as we approach and pass the average interval
	probability := 1.0 / (1.0 + math.Exp(-2*(ratio-1)))

	// Cap probability
	return math.Min(probability, 0.95)
}
