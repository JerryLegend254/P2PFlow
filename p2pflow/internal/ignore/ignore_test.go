package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIgnoreMatcher_ShouldIgnore(t *testing.T) {
	tests := []struct {
		name      string
		patterns  []string
		path      string
		isDir     bool
		wantIgnore bool
	}{
		{
			name:      "ignore .git directory",
			patterns:  []string{".git/"},
			path:      ".git",
			isDir:     true,
			wantIgnore: true,
		},
		{
			name:      "ignore node_modules directory",
			patterns:  []string{"node_modules/"},
			path:      "node_modules",
			isDir:     true,
			wantIgnore: true,
		},
		{
			name:      "ignore .env file",
			patterns:  []string{".env"},
			path:      ".env",
			isDir:     false,
			wantIgnore: true,
		},
		{
			name:      "ignore all log files",
			patterns:  []string{"*.log"},
			path:      "app.log",
			isDir:     false,
			wantIgnore: true,
		},
		{
			name:      "don't ignore regular file",
			patterns:  []string{"*.log"},
			path:      "main.go",
			isDir:     false,
			wantIgnore: false,
		},
		{
			name:      "ignore nested path with wildcard",
			patterns:  []string{"*.log"},
			path:      "logs/app.log",
			isDir:     false,
			wantIgnore: true,
		},
		{
			name:      "negation pattern",
			patterns:  []string{"*.log", "!important.log"},
			path:      "important.log",
			isDir:     false,
			wantIgnore: false,
		},
		{
			name:      "negation doesn't apply to other files",
			patterns:  []string{"*.log", "!important.log"},
			path:      "debug.log",
			isDir:     false,
			wantIgnore: true,
		},
		{
			name:      "ignore directory only pattern on file",
			patterns:  []string{"test/"},
			path:      "test",
			isDir:     false,
			wantIgnore: false,
		},
		{
			name:      "ignore directory only pattern on dir",
			patterns:  []string{"test/"},
			path:      "test",
			isDir:     true,
			wantIgnore: true,
		},
		{
			name:      "path-specific pattern",
			patterns:  []string{"src/test/"},
			path:      "src/test",
			isDir:     true,
			wantIgnore: true,
		},
		{
			name:      "path-specific pattern doesn't match different path",
			patterns:  []string{"src/test/"},
			path:      "lib/test",
			isDir:     true,
			wantIgnore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			im := NewIgnoreMatcher("/tmp/test")
			for _, pattern := range tt.patterns {
				im.AddPattern(pattern)
			}

			got := im.ShouldIgnore(tt.path, tt.isDir)
			if got != tt.wantIgnore {
				t.Errorf("ShouldIgnore() = %v, want %v for path %s", got, tt.wantIgnore, tt.path)
			}
		})
	}
}

func TestIgnoreMatcher_LoadFromFile(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "p2pflow-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a .p2pignore file
	ignoreFile := filepath.Join(tempDir, ".p2pignore")
	content := `# Comment line
*.log
.env
node_modules/

# Another comment
!important.log
`
	if err := os.WriteFile(ignoreFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write ignore file: %v", err)
	}

	// Load patterns
	im := NewIgnoreMatcher(tempDir)
	if err := im.LoadFromFile(ignoreFile); err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	// Test patterns were loaded
	patterns := im.GetPatterns()
	expectedPatterns := []string{"*.log", ".env", "node_modules/", "!important.log"}

	if len(patterns) != len(expectedPatterns) {
		t.Errorf("Expected %d patterns, got %d", len(expectedPatterns), len(patterns))
	}

	// Test that patterns work
	tests := []struct {
		path      string
		isDir     bool
		wantIgnore bool
	}{
		{"app.log", false, true},
		{".env", false, true},
		{"node_modules", true, true},
		{"important.log", false, false},
		{"main.go", false, false},
	}

	for _, tt := range tests {
		got := im.ShouldIgnore(tt.path, tt.isDir)
		if got != tt.wantIgnore {
			t.Errorf("After loading file, ShouldIgnore(%s) = %v, want %v", tt.path, got, tt.wantIgnore)
		}
	}
}

func TestIgnoreMatcher_AddDefaultPatterns(t *testing.T) {
	im := NewIgnoreMatcher("/tmp/test")
	im.AddDefaultPatterns()

	// Test some default patterns
	tests := []struct {
		path      string
		isDir     bool
		wantIgnore bool
	}{
		{".git", true, true},
		{".collab", true, true},
		{".DS_Store", false, true},
		{"node_modules", true, true},
		{".env", false, true},
		{"app.log", false, true},
		{"main.go", false, false},
	}

	for _, tt := range tests {
		got := im.ShouldIgnore(tt.path, tt.isDir)
		if got != tt.wantIgnore {
			t.Errorf("With default patterns, ShouldIgnore(%s) = %v, want %v", tt.path, got, tt.wantIgnore)
		}
	}
}

func TestIgnoreMatcher_EmptyPatterns(t *testing.T) {
	im := NewIgnoreMatcher("/tmp/test")

	// With no patterns, nothing should be ignored
	tests := []string{"main.go", ".git", "node_modules", ".env"}
	for _, path := range tests {
		if im.ShouldIgnore(path, false) {
			t.Errorf("With no patterns, ShouldIgnore(%s) should be false", path)
		}
	}
}

func TestIgnoreMatcher_RelativePaths(t *testing.T) {
	im := NewIgnoreMatcher("/tmp/test")
	im.AddPattern("*.log")

	tests := []struct {
		path      string
		wantIgnore bool
	}{
		{"app.log", true},
		{"logs/app.log", true},
		{"./logs/debug.log", true},
		{"main.go", false},
	}

	for _, tt := range tests {
		got := im.ShouldIgnore(tt.path, false)
		if got != tt.wantIgnore {
			t.Errorf("ShouldIgnore(%s) = %v, want %v", tt.path, got, tt.wantIgnore)
		}
	}
}
