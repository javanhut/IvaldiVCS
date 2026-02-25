package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewPatternCache(t *testing.T) {
	patterns := []string{
		"*.log",
		"node_modules/",
		"target/",
		".DS_Store",
		"**/*.bak",
	}

	cache := NewPatternCache(patterns)

	if len(cache.patterns) != 5 {
		t.Errorf("Expected 5 patterns, got %d", len(cache.patterns))
	}
	if len(cache.dirPatterns) != 2 {
		t.Errorf("Expected 2 dir patterns, got %d", len(cache.dirPatterns))
	}
	if len(cache.globPatterns) != 1 {
		t.Errorf("Expected 1 glob pattern, got %d", len(cache.globPatterns))
	}
	if len(cache.doubleStarPats) != 1 {
		t.Errorf("Expected 1 double-star pattern, got %d", len(cache.doubleStarPats))
	}
	if len(cache.literalMatches) != 1 {
		t.Errorf("Expected 1 literal match, got %d", len(cache.literalMatches))
	}
}

func TestIsIgnored(t *testing.T) {
	patterns := []string{
		"*.log",
		"target/",
		"node_modules/",
		"**/*.bak",
		".DS_Store",
		"build/",
	}

	cache := NewPatternCache(patterns)

	tests := []struct {
		path     string
		expected bool
	}{
		// Literal match
		{".DS_Store", true},

		// Glob patterns
		{"app.log", true},
		{"src/debug.log", true},

		// Directory patterns
		{"target/release/deps/foo", true},
		{"target/debug/build/bar", true},
		{"node_modules/express/index.js", true},
		{"build/output.js", true},

		// ** glob patterns
		{"file.bak", true},
		{"src/deep/file.bak", true},

		// Not ignored
		{"src/main.go", false},
		{"README.md", false},
		{"Cargo.toml", false},

		// .ivaldiignore should never be ignored
		{".ivaldiignore", false},
		{"subdir/.ivaldiignore", false},
	}

	for _, tc := range tests {
		result := cache.IsIgnored(tc.path)
		if result != tc.expected {
			t.Errorf("IsIgnored(%q) = %v, want %v", tc.path, result, tc.expected)
		}
	}
}

func TestIsDirIgnored(t *testing.T) {
	patterns := []string{
		"target/",
		"node_modules/",
		"build/",
		"*.log",
	}

	cache := NewPatternCache(patterns)

	tests := []struct {
		path     string
		expected bool
	}{
		{"target", true},
		{"node_modules", true},
		{"build", true},
		{"target/release", true},
		{"target/debug/deps", true},
		{"src", false},
		{"internal", false},
	}

	for _, tc := range tests {
		result := cache.IsDirIgnored(tc.path)
		if result != tc.expected {
			t.Errorf("IsDirIgnored(%q) = %v, want %v", tc.path, result, tc.expected)
		}
	}
}

func TestLoadPatterns(t *testing.T) {
	tempDir := t.TempDir()

	// Test with no .ivaldiignore file
	patterns, err := LoadPatterns(tempDir)
	if err != nil {
		t.Fatalf("LoadPatterns failed with no ignore file: %v", err)
	}
	if len(patterns) != 0 {
		t.Errorf("Expected 0 patterns with no ignore file, got %d", len(patterns))
	}

	// Create .ivaldiignore file
	content := `# Build output
target/
build/

# Dependencies
node_modules/

# Logs
*.log

# OS files
.DS_Store
`
	ignoreFile := filepath.Join(tempDir, ".ivaldiignore")
	if err := os.WriteFile(ignoreFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write .ivaldiignore: %v", err)
	}

	patterns, err = LoadPatterns(tempDir)
	if err != nil {
		t.Fatalf("LoadPatterns failed: %v", err)
	}

	expected := []string{"target/", "build/", "node_modules/", "*.log", ".DS_Store"}
	if len(patterns) != len(expected) {
		t.Fatalf("Expected %d patterns, got %d: %v", len(expected), len(patterns), patterns)
	}

	for i, p := range expected {
		if patterns[i] != p {
			t.Errorf("Pattern %d: expected %q, got %q", i, p, patterns[i])
		}
	}
}

func TestLoadPatternCache(t *testing.T) {
	tempDir := t.TempDir()

	// Should return non-nil cache even with no ignore file
	cache, err := LoadPatternCache(tempDir)
	if err != nil {
		t.Fatalf("LoadPatternCache failed: %v", err)
	}
	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}

	// Should not ignore anything with empty patterns
	if cache.IsIgnored("anything.go") {
		t.Error("Empty cache should not ignore any file")
	}
}

func TestEmptyPatternCache(t *testing.T) {
	cache := NewPatternCache(nil)

	if cache.IsIgnored("foo.go") {
		t.Error("Nil-patterns cache should not ignore anything")
	}
	if cache.IsDirIgnored("target") {
		t.Error("Nil-patterns cache should not ignore any directory")
	}
}
