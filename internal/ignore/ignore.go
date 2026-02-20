// Package ignore provides shared ignore-pattern matching for Ivaldi.
//
// It loads patterns from .ivaldiignore files and provides efficient
// matching via PatternCache, used by both CLI commands and the
// workspace scanner.
package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DefaultPatterns are built-in ignore patterns for common VCS and tool directories.
// These are always applied, even without an .ivaldiignore file.
var DefaultPatterns = []string{
	".git/",
	".svn/",
	".hg/",
	".fossil/",
	".claude/",
}

// doubleStarPattern holds a pre-split ** glob pattern.
type doubleStarPattern struct {
	prefix string // Part before **
	suffix string // Part after **
}

// PatternCache holds pre-compiled ignore patterns for fast matching.
type PatternCache struct {
	patterns         []string
	dirPatterns      []string             // Patterns ending with /
	globPatterns     []string             // Patterns with wildcards (no **)
	doubleStarPats   []doubleStarPattern  // Pre-split ** patterns
	literalMatches   map[string]bool      // Exact-match patterns
}

// NewPatternCache creates a PatternCache from a list of patterns.
func NewPatternCache(patterns []string) *PatternCache {
	cache := &PatternCache{
		patterns:       patterns,
		literalMatches: make(map[string]bool),
	}

	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/") {
			cache.dirPatterns = append(cache.dirPatterns, strings.TrimSuffix(pattern, "/"))
		} else if strings.Contains(pattern, "**") {
			// Pre-split ** patterns at cache creation time
			parts := strings.SplitN(pattern, "**", 2)
			if len(parts) == 2 {
				cache.doubleStarPats = append(cache.doubleStarPats, doubleStarPattern{
					prefix: strings.TrimPrefix(parts[0], "/"),
					suffix: strings.TrimPrefix(parts[1], "/"),
				})
			}
		} else if strings.ContainsAny(pattern, "*?[") {
			cache.globPatterns = append(cache.globPatterns, pattern)
		} else {
			cache.literalMatches[pattern] = true
		}
	}

	return cache
}

// IsIgnored checks if a file path matches any cached pattern.
// .ivaldiignore itself is never ignored.
func (pc *PatternCache) IsIgnored(path string) bool {
	if path == ".ivaldiignore" || filepath.Base(path) == ".ivaldiignore" {
		return false
	}

	baseName := filepath.Base(path)

	// Fast literal match check
	if pc.literalMatches[path] || pc.literalMatches[baseName] {
		return true
	}

	// Check directory patterns
	for _, dirPattern := range pc.dirPatterns {
		if strings.HasPrefix(path, dirPattern+"/") || path == dirPattern {
			return true
		}
	}

	// Check simple glob patterns (no **)
	for _, pattern := range pc.globPatterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, baseName); matched {
			return true
		}
	}

	// Check pre-compiled ** patterns
	for _, dsp := range pc.doubleStarPats {
		if dsp.prefix != "" && !strings.HasPrefix(path, dsp.prefix) {
			continue
		}
		if dsp.suffix != "" {
			if matched, _ := filepath.Match(dsp.suffix, baseName); matched {
				return true
			}
		} else if dsp.prefix == "" {
			// Pattern is just "**" — matches everything
			return true
		} else {
			// Pattern is "prefix/**" — matches anything under prefix
			return true
		}
	}

	return false
}

// IsDirIgnored checks if a directory path matches any cached directory pattern.
// This is used for early pruning during directory walks.
func (pc *PatternCache) IsDirIgnored(dirPath string) bool {
	baseName := filepath.Base(dirPath)

	// Check literal matches (directory name)
	if pc.literalMatches[dirPath] || pc.literalMatches[baseName] {
		return true
	}

	// Check directory patterns
	for _, dirPattern := range pc.dirPatterns {
		if dirPath == dirPattern || strings.HasPrefix(dirPath, dirPattern+"/") {
			return true
		}
		// Also match if the directory basename matches
		if baseName == dirPattern {
			return true
		}
	}

	return false
}

// LoadPatterns reads ignore patterns from the .ivaldiignore file in workDir.
func LoadPatterns(workDir string) ([]string, error) {
	ignoreFile := filepath.Join(workDir, ".ivaldiignore")
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		return []string{}, nil
	}

	file, err := os.Open(ignoreFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns, scanner.Err()
}

// LoadPatternCache is a convenience function that loads patterns and creates a cache.
// It prepends DefaultPatterns before any user-defined patterns from .ivaldiignore.
// Returns a non-nil cache even on error (with default patterns only).
func LoadPatternCache(workDir string) (*PatternCache, error) {
	patterns, err := LoadPatterns(workDir)
	combined := make([]string, 0, len(DefaultPatterns)+len(patterns))
	combined = append(combined, DefaultPatterns...)
	combined = append(combined, patterns...)
	if err != nil {
		return NewPatternCache(DefaultPatterns), err
	}
	return NewPatternCache(combined), nil
}
