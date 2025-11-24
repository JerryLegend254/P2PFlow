package analytics

import (
	"sort"
	"sync"
	"time"
)

// PrefetchSuggestion represents a file that should be prefetched
type PrefetchSuggestion struct {
	FilePath   string    `json:"file_path"`
	Priority   float64   `json:"priority"`   // 0.0 to 1.0, higher = more important
	Confidence float64   `json:"confidence"` // 0.0 to 1.0, how sure we are
	Reason     string    `json:"reason"`
	Timestamp  time.Time `json:"timestamp"`
	EstimatedAccessTime time.Time `json:"estimated_access_time,omitempty"`
}

// PrefetchEngine manages intelligent file prefetching
type PrefetchEngine struct {
	predictor       *Predictor
	enabled         bool
	suggestions     map[string]*PrefetchSuggestion // filePath -> suggestion
	suggestionsMu   sync.RWMutex
	maxSuggestions  int
	refreshInterval time.Duration
	lastRefresh     time.Time
}

// NewPrefetchEngine creates a new prefetch engine
func NewPrefetchEngine(predictor *Predictor, enabled bool) *PrefetchEngine {
	return &PrefetchEngine{
		predictor:       predictor,
		enabled:         enabled,
		suggestions:     make(map[string]*PrefetchSuggestion),
		maxSuggestions:  20,
		refreshInterval: 5 * time.Minute,
		lastRefresh:     time.Now(),
	}
}

// GetSuggestions returns prefetch suggestions based on current context
func (e *PrefetchEngine) GetSuggestions(currentContext []string, maxSuggestions int) []PrefetchSuggestion {
	if !e.enabled {
		return nil
	}

	e.suggestionsMu.Lock()
	defer e.suggestionsMu.Unlock()

	// Refresh suggestions if needed
	if time.Since(e.lastRefresh) > e.refreshInterval {
		e.refreshSuggestions(currentContext)
	}

	// Convert map to slice
	suggestions := make([]PrefetchSuggestion, 0, len(e.suggestions))
	for _, suggestion := range e.suggestions {
		suggestions = append(suggestions, *suggestion)
	}

	// Sort by priority (descending)
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority > suggestions[j].Priority
	})

	// Return top N
	if maxSuggestions > 0 && maxSuggestions < len(suggestions) {
		return suggestions[:maxSuggestions]
	}

	return suggestions
}

// refreshSuggestions updates the list of prefetch suggestions
func (e *PrefetchEngine) refreshSuggestions(currentContext []string) {
	newSuggestions := make(map[string]*PrefetchSuggestion)

	// Strategy 1: Predict based on current context
	if len(currentContext) > 0 {
		for _, currentFile := range currentContext {
			predictions := e.predictor.PredictNext(currentFile, 5)
			for _, pred := range predictions {
				e.addOrUpdateSuggestion(newSuggestions, pred)
			}
		}
	}

	// Strategy 2: Time-based predictions (files likely to be accessed soon)
	timePredictions := e.predictor.predictByTimePattern()
	for _, pred := range timePredictions {
		e.addOrUpdateSuggestion(newSuggestions, pred)
	}

	// Strategy 3: Day-of-week predictions
	dayPredictions := e.predictor.PredictByDayOfWeek(time.Now().Weekday(), 5)
	for _, pred := range dayPredictions {
		e.addOrUpdateSuggestion(newSuggestions, pred)
	}

	// Strategy 4: High-probability files (files likely to be accessed in next few hours)
	e.addHighProbabilityFiles(newSuggestions)

	// Update suggestions
	e.suggestions = newSuggestions
	e.lastRefresh = time.Now()

	// Trim to max suggestions
	if len(e.suggestions) > e.maxSuggestions {
		e.trimSuggestions()
	}
}

// addOrUpdateSuggestion adds or updates a prefetch suggestion
func (e *PrefetchEngine) addOrUpdateSuggestion(suggestions map[string]*PrefetchSuggestion, pred Prediction) {
	existing, exists := suggestions[pred.FilePath]

	// Calculate priority (combination of confidence and other factors)
	priority := e.calculatePriority(pred)

	if !exists || priority > existing.Priority {
		suggestions[pred.FilePath] = &PrefetchSuggestion{
			FilePath:   pred.FilePath,
			Priority:   priority,
			Confidence: pred.Confidence,
			Reason:     pred.Reason,
			Timestamp:  time.Now(),
		}
	}
}

// calculatePriority calculates the priority for a prediction
func (e *PrefetchEngine) calculatePriority(pred Prediction) float64 {
	// Base priority is the confidence
	priority := pred.Confidence

	// Boost priority for frequently accessed files
	fileStats := e.predictor.tracker.GetFileStatistics(pred.FilePath)
	if fileStats != nil {
		// Files accessed more frequently get higher priority
		daysSinceFirst := time.Since(fileStats.FirstAccess).Hours() / 24
		if daysSinceFirst < 1 {
			daysSinceFirst = 1
		}
		accessesPerDay := float64(fileStats.TotalAccesses) / daysSinceFirst

		// Normalize to 0-0.2 range and add to priority
		frequencyBoost := (accessesPerDay / 10.0) * 0.2
		if frequencyBoost > 0.2 {
			frequencyBoost = 0.2
		}
		priority += frequencyBoost

		// Boost priority if file hasn't been accessed in a while (might be due soon)
		timeSinceLastAccess := time.Since(fileStats.LastAccess)
		avgTimeBetweenAccesses := time.Duration(daysSinceFirst*24) * time.Hour / time.Duration(fileStats.TotalAccesses)

		if timeSinceLastAccess > avgTimeBetweenAccesses {
			// Overdue, boost priority
			overdueBoost := 0.1
			priority += overdueBoost
		}
	}

	// Cap priority at 1.0
	if priority > 1.0 {
		priority = 1.0
	}

	return priority
}

// addHighProbabilityFiles adds files with high access probability
func (e *PrefetchEngine) addHighProbabilityFiles(suggestions map[string]*PrefetchSuggestion) {
	stats := e.predictor.tracker.GetStatistics()

	// Check top accessed files
	for _, summary := range stats.MostAccessedFiles {
		probability := e.predictor.CalculateAccessProbability(summary.FilePath, 6) // Next 6 hours

		if probability >= 0.7 {
			pred := Prediction{
				FilePath:   summary.FilePath,
				Confidence: probability,
				Reason:     "High probability of access in next 6 hours",
				Score:      probability,
				Timestamp:  time.Now(),
			}

			e.addOrUpdateSuggestion(suggestions, pred)
		}
	}
}

// trimSuggestions reduces the number of suggestions to maxSuggestions
func (e *PrefetchEngine) trimSuggestions() {
	// Convert to slice
	suggestions := make([]*PrefetchSuggestion, 0, len(e.suggestions))
	for _, suggestion := range e.suggestions {
		suggestions = append(suggestions, suggestion)
	}

	// Sort by priority
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority > suggestions[j].Priority
	})

	// Keep only top maxSuggestions
	newSuggestions := make(map[string]*PrefetchSuggestion)
	for i := 0; i < e.maxSuggestions && i < len(suggestions); i++ {
		suggestion := suggestions[i]
		newSuggestions[suggestion.FilePath] = suggestion
	}

	e.suggestions = newSuggestions
}

// ShouldPrefetch determines if a file should be prefetched
func (e *PrefetchEngine) ShouldPrefetch(filePath string, minPriority float64) bool {
	if !e.enabled {
		return false
	}

	e.suggestionsMu.RLock()
	defer e.suggestionsMu.RUnlock()

	suggestion, exists := e.suggestions[filePath]
	if !exists {
		return false
	}

	return suggestion.Priority >= minPriority
}

// MarkPrefetched marks a file as having been prefetched
func (e *PrefetchEngine) MarkPrefetched(filePath string) {
	e.suggestionsMu.Lock()
	defer e.suggestionsMu.Unlock()

	// Remove from suggestions since it's now prefetched
	delete(e.suggestions, filePath)
}

// GetTopSuggestion returns the highest priority suggestion
func (e *PrefetchEngine) GetTopSuggestion() *PrefetchSuggestion {
	if !e.enabled {
		return nil
	}

	e.suggestionsMu.RLock()
	defer e.suggestionsMu.RUnlock()

	var topSuggestion *PrefetchSuggestion
	maxPriority := 0.0

	for _, suggestion := range e.suggestions {
		if suggestion.Priority > maxPriority {
			maxPriority = suggestion.Priority
			topSuggestion = suggestion
		}
	}

	if topSuggestion != nil {
		// Return a copy
		copy := *topSuggestion
		return &copy
	}

	return nil
}

// SetEnabled enables or disables prefetching
func (e *PrefetchEngine) SetEnabled(enabled bool) {
	e.suggestionsMu.Lock()
	defer e.suggestionsMu.Unlock()

	e.enabled = enabled
}

// IsEnabled returns whether prefetching is enabled
func (e *PrefetchEngine) IsEnabled() bool {
	e.suggestionsMu.RLock()
	defer e.suggestionsMu.RUnlock()

	return e.enabled
}

// SetMaxSuggestions sets the maximum number of suggestions to track
func (e *PrefetchEngine) SetMaxSuggestions(max int) {
	e.suggestionsMu.Lock()
	defer e.suggestionsMu.Unlock()

	e.maxSuggestions = max
	if len(e.suggestions) > max {
		e.trimSuggestions()
	}
}

// SetRefreshInterval sets how often suggestions are refreshed
func (e *PrefetchEngine) SetRefreshInterval(interval time.Duration) {
	e.suggestionsMu.Lock()
	defer e.suggestionsMu.Unlock()

	e.refreshInterval = interval
}

// ForceRefresh forces an immediate refresh of suggestions
func (e *PrefetchEngine) ForceRefresh(currentContext []string) {
	e.suggestionsMu.Lock()
	defer e.suggestionsMu.Unlock()

	e.refreshSuggestions(currentContext)
}

// GetSuggestionCount returns the current number of suggestions
func (e *PrefetchEngine) GetSuggestionCount() int {
	e.suggestionsMu.RLock()
	defer e.suggestionsMu.RUnlock()

	return len(e.suggestions)
}
