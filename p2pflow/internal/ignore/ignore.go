package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// IgnoreMatcher handles pattern matching for file filtering
type IgnoreMatcher struct {
	patterns []pattern
	rootPath string
}

// pattern represents a single ignore pattern
type pattern struct {
	raw       string // Original pattern string
	isNegated bool   // Pattern starts with !
	isDir     bool   // Pattern ends with /
	hasSlash  bool   // Pattern contains /
	parts     []string
}

// NewIgnoreMatcher creates a new ignore matcher
func NewIgnoreMatcher(rootPath string) *IgnoreMatcher {
	return &IgnoreMatcher{
		patterns: make([]pattern, 0),
		rootPath: rootPath,
	}
}

// LoadFromFile loads ignore patterns from a .p2pignore file
func (im *IgnoreMatcher) LoadFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, not an error
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		im.AddPattern(line)
	}

	return scanner.Err()
}

// AddPattern adds a single ignore pattern
func (im *IgnoreMatcher) AddPattern(patternStr string) {
	patternStr = strings.TrimSpace(patternStr)
	if patternStr == "" {
		return
	}

	p := pattern{raw: patternStr}

	// Check for negation
	if strings.HasPrefix(patternStr, "!") {
		p.isNegated = true
		patternStr = strings.TrimPrefix(patternStr, "!")
	}

	// Check for directory marker
	if strings.HasSuffix(patternStr, "/") {
		p.isDir = true
		patternStr = strings.TrimSuffix(patternStr, "/")
	}

	// Check if pattern contains slash (path-specific)
	p.hasSlash = strings.Contains(patternStr, "/")

	// Split into parts for matching
	p.parts = strings.Split(patternStr, "/")

	im.patterns = append(im.patterns, p)
}

// AddDefaultPatterns adds common ignore patterns
func (im *IgnoreMatcher) AddDefaultPatterns() {
	defaults := []string{
		".git/",
		".collab/",
		".DS_Store",
		"*.swp",
		"*.swo",
		"*~",
		".*.sw?",
		"node_modules/",
		".env",
		".env.local",
		"*.log",
		".vscode/",
		".idea/",
	}

	for _, pattern := range defaults {
		im.AddPattern(pattern)
	}
}

// ShouldIgnore checks if a file path should be ignored
func (im *IgnoreMatcher) ShouldIgnore(path string, isDir bool) bool {
	// Normalize path
	path = filepath.Clean(path)

	// Make path relative to root
	if filepath.IsAbs(path) && im.rootPath != "" {
		relPath, err := filepath.Rel(im.rootPath, path)
		if err == nil {
			path = relPath
		}
	}

	// Convert to forward slashes for consistent matching
	path = filepath.ToSlash(path)

	// Track if ignored (default: not ignored)
	ignored := false

	// Apply patterns in order
	for _, p := range im.patterns {
		matches := im.matchPattern(p, path, isDir)

		if matches {
			if p.isNegated {
				// Negation pattern - don't ignore
				ignored = false
			} else {
				// Regular pattern - ignore
				ignored = true
			}
		}
	}

	return ignored
}

// matchPattern checks if a pattern matches a path
func (im *IgnoreMatcher) matchPattern(p pattern, path string, isDir bool) bool {
	// If pattern is for directories only, check if path is a directory
	if p.isDir && !isDir {
		return false
	}

	pathParts := strings.Split(path, "/")

	if p.hasSlash {
		// Path-specific pattern - match from root
		return im.matchPathParts(p.parts, pathParts)
	}

	// Name-only pattern - match basename or any path component
	// Try matching against the full path first
	if im.matchPathParts(p.parts, pathParts) {
		return true
	}

	// Also try matching just the basename
	if len(pathParts) > 0 {
		basename := pathParts[len(pathParts)-1]
		if len(p.parts) == 1 {
			return im.matchGlob(p.parts[0], basename)
		}
	}

	return false
}

// matchPathParts matches pattern parts against path parts
func (im *IgnoreMatcher) matchPathParts(patternParts, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}

	// Handle ** wildcard (match any number of directories)
	for i, part := range patternParts {
		if part == "**" {
			if i == len(patternParts)-1 {
				// ** at end matches everything
				return true
			}
			// Try matching remaining pattern at different positions
			for j := i; j <= len(pathParts); j++ {
				if im.matchPathParts(patternParts[i+1:], pathParts[j:]) {
					return true
				}
			}
			return false
		}
	}

	// No ** wildcard - match sequentially
	if len(patternParts) > len(pathParts) {
		return false
	}

	// Check if pattern can match from the beginning
	for i := 0; i < len(patternParts); i++ {
		if i >= len(pathParts) {
			return false
		}
		if !im.matchGlob(patternParts[i], pathParts[i]) {
			return false
		}
	}

	// Pattern matched up to its length
	// If pattern length equals path length, it's an exact match
	// If pattern is shorter, it matches as a prefix
	return len(patternParts) == len(pathParts) ||
		(len(patternParts) < len(pathParts) && !strings.Contains(strings.Join(patternParts, "/"), "/"))
}

// matchGlob performs simple glob matching
func (im *IgnoreMatcher) matchGlob(pattern, name string) bool {
	// Simple glob implementation
	// Supports * (any characters) and ? (single character)

	if pattern == name {
		return true
	}

	if pattern == "*" {
		return true
	}

	// Use filepath.Match for glob matching
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return false
	}

	return matched
}

// GetPatterns returns all loaded patterns
func (im *IgnoreMatcher) GetPatterns() []string {
	patterns := make([]string, len(im.patterns))
	for i, p := range im.patterns {
		patterns[i] = p.raw
	}
	return patterns
}
