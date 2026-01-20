package cli

import (
	"testing"
)

// BenchmarkPatternCache benchmarks the pattern cache matching
func BenchmarkPatternCache(b *testing.B) {
	patterns := []string{
		"*.log",
		"*.tmp",
		"node_modules/",
		"vendor/",
		"dist/",
		"build/",
		"**/*.bak",
		"**/*.swp",
		".git/",
		".DS_Store",
		"Thumbs.db",
		"coverage/",
		"*.pyc",
		"__pycache__/",
	}

	testPaths := []string{
		"src/main.go",
		"src/utils/helper.go",
		"node_modules/express/index.js",
		"dist/bundle.js",
		"README.md",
		"internal/parser/parser.go",
		"test/integration_test.go",
		"docs/api.md",
		"config.yaml",
		"main.log",
		"debug.tmp",
		"vendor/github.com/example/pkg/file.go",
	}

	b.Run("PatternCache", func(b *testing.B) {
		cache := NewPatternCache(patterns)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, path := range testPaths {
				cache.IsIgnored(path)
			}
		}
	})

	b.Run("OriginalPatternMatching", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, path := range testPaths {
				isFileIgnored(path, patterns)
			}
		}
	})
}

// BenchmarkPatternCacheLargePatternSet benchmarks with many patterns
func BenchmarkPatternCacheLargePatternSet(b *testing.B) {
	// Generate a large pattern set
	patterns := make([]string, 100)
	for i := 0; i < 50; i++ {
		patterns[i] = "dir" + string(rune('a'+i%26)) + "/"
	}
	for i := 50; i < 100; i++ {
		patterns[i] = "*." + string(rune('a'+i%26)) + "xt"
	}

	testPaths := make([]string, 100)
	for i := 0; i < 100; i++ {
		testPaths[i] = "src/pkg" + string(rune('a'+i%26)) + "/file" + string(rune('0'+i%10)) + ".go"
	}

	b.Run("PatternCache_100Patterns", func(b *testing.B) {
		cache := NewPatternCache(patterns)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, path := range testPaths {
				cache.IsIgnored(path)
			}
		}
	})

	b.Run("Original_100Patterns", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, path := range testPaths {
				isFileIgnored(path, patterns)
			}
		}
	})
}

// TestPatternCacheCorrectness verifies pattern cache matches original behavior
func TestPatternCacheCorrectness(t *testing.T) {
	patterns := []string{
		"*.log",
		"node_modules/",
		"dist/",
		"**/*.bak",
		".DS_Store",
	}

	testCases := []struct {
		path     string
		expected bool
	}{
		{"main.log", true},
		{"src/main.go", false},
		{"node_modules/express/index.js", true},
		{"dist/bundle.js", true},
		{".DS_Store", true},
		{"README.md", false},
		{"file.bak", true},
		{".ivaldiignore", false}, // Should never be ignored
	}

	cache := NewPatternCache(patterns)

	for _, tc := range testCases {
		cacheResult := cache.IsIgnored(tc.path)
		originalResult := isFileIgnored(tc.path, patterns)

		if cacheResult != originalResult {
			t.Errorf("Mismatch for path %q: cache=%v, original=%v", tc.path, cacheResult, originalResult)
		}

		if cacheResult != tc.expected {
			t.Errorf("Path %q: expected %v, got %v", tc.path, tc.expected, cacheResult)
		}
	}
}
