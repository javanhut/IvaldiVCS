package refs

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/store"
)

// TimelineType represents different types of timelines
type TimelineType string

const (
	LocalTimeline  TimelineType = "local"
	RemoteTimeline TimelineType = "remote"
	TagTimeline    TimelineType = "tag"
)

// Timeline represents a branch or tag reference
type Timeline struct {
	Name         string       `json:"name"`
	Type         TimelineType `json:"type"`
	Blake3Hash   [32]byte     `json:"blake3_hash"`
	SHA256Hash   [32]byte     `json:"sha256_hash"`
	GitSHA1Hash  string       `json:"git_sha1_hash,omitempty"` // Original Git hash if converted
	LastUpdated  time.Time    `json:"last_updated"`
	Description  string       `json:"description,omitempty"`
}

// RefsManager handles timeline and reference management
type RefsManager struct {
	ivaldiDir string
	refsDir   string
	db        *store.DB
}

// NewRefsManager creates a new refs manager
func NewRefsManager(ivaldiDir string) (*RefsManager, error) {
	refsDir := filepath.Join(ivaldiDir, "refs")
	if err := os.MkdirAll(refsDir, 0755); err != nil {
		return nil, fmt.Errorf("create refs dir: %w", err)
	}

	// Create subdirectories for different ref types
	for _, subdir := range []string{"heads", "remotes", "tags"} {
		if err := os.MkdirAll(filepath.Join(refsDir, subdir), 0755); err != nil {
			return nil, fmt.Errorf("create refs subdir %s: %w", subdir, err)
		}
	}

	dbPath := filepath.Join(ivaldiDir, "objects.db")
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	return &RefsManager{
		ivaldiDir: ivaldiDir,
		refsDir:   refsDir,
		db:        db,
	}, nil
}

// Close closes the refs manager
func (rm *RefsManager) Close() error {
	return rm.db.Close()
}

// CreateTimeline creates a new timeline (branch)
func (rm *RefsManager) CreateTimeline(name string, timelineType TimelineType, blake3Hash [32]byte, sha256Hash [32]byte, gitSHA1Hash string, description string) error {
	timeline := Timeline{
		Name:         name,
		Type:         timelineType,
		Blake3Hash:   blake3Hash,
		SHA256Hash:   sha256Hash,
		GitSHA1Hash:  gitSHA1Hash,
		LastUpdated:  time.Now(),
		Description:  description,
	}

	return rm.writeTimeline(timeline)
}

// UpdateTimeline updates an existing timeline
func (rm *RefsManager) UpdateTimeline(name string, timelineType TimelineType, blake3Hash [32]byte, sha256Hash [32]byte, gitSHA1Hash string) error {
	timeline := Timeline{
		Name:         name,
		Type:         timelineType,
		Blake3Hash:   blake3Hash,
		SHA256Hash:   sha256Hash,
		GitSHA1Hash:  gitSHA1Hash,
		LastUpdated:  time.Now(),
	}

	return rm.writeTimeline(timeline)
}

// GetTimeline retrieves a timeline by name and type
func (rm *RefsManager) GetTimeline(name string, timelineType TimelineType) (*Timeline, error) {
	refPath := rm.getRefPath(name, timelineType)
	data, err := os.ReadFile(refPath)
	if err != nil {
		return nil, fmt.Errorf("read timeline %s: %w", name, err)
	}

	// Parse the ref file (format: blake3_hex sha256_hex git_sha1_hex timestamp description)
	parts := strings.Fields(string(data))
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid ref file format for %s", name)
	}

	blake3Hash, err := hex.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode blake3 hash: %w", err)
	}

	sha256Hash, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode sha256 hash: %w", err)
	}

	var blake3Array [32]byte
	var sha256Array [32]byte
	copy(blake3Array[:], blake3Hash)
	copy(sha256Array[:], sha256Hash)

	timeline := &Timeline{
		Name:        name,
		Type:        timelineType,
		Blake3Hash:  blake3Array,
		SHA256Hash:  sha256Array,
		LastUpdated: time.Now(), // Would parse from parts[3] in real implementation
	}

	if len(parts) > 2 {
		timeline.GitSHA1Hash = parts[2]
	}
	if len(parts) > 4 {
		timeline.Description = strings.Join(parts[4:], " ")
	}

	return timeline, nil
}

// ListTimelines lists all timelines of a specific type
func (rm *RefsManager) ListTimelines(timelineType TimelineType) ([]Timeline, error) {
	var timelines []Timeline
	subdir := rm.getSubdir(timelineType)
	searchDir := filepath.Join(rm.refsDir, subdir)

	err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(searchDir, path)
		if err != nil {
			return err
		}

		// Convert file path back to timeline name
		timelineName := strings.ReplaceAll(relPath, string(filepath.Separator), "/")
		
		timeline, err := rm.GetTimeline(timelineName, timelineType)
		if err != nil {
			return err
		}

		timelines = append(timelines, *timeline)
		return nil
	})

	return timelines, err
}

// GetCurrentTimeline gets the current active timeline
func (rm *RefsManager) GetCurrentTimeline() (string, error) {
	headPath := filepath.Join(rm.ivaldiDir, "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", fmt.Errorf("read HEAD: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/"), nil
	}

	return "", fmt.Errorf("HEAD is detached or invalid format")
}

// SetCurrentTimeline sets the current active timeline
func (rm *RefsManager) SetCurrentTimeline(name string) error {
	headPath := filepath.Join(rm.ivaldiDir, "HEAD")
	content := fmt.Sprintf("ref: refs/heads/%s\n", name)
	return os.WriteFile(headPath, []byte(content), 0644)
}

// MapGitHashToBlake3 creates a mapping from Git SHA1 hash to Blake3 hash
func (rm *RefsManager) MapGitHashToBlake3(gitSHA1 string, blake3Hash [32]byte, sha256Hash [32]byte) error {
	return rm.db.PutGitMapping(gitSHA1, blake3Hash, sha256Hash)
}

// LookupByGitHash finds Ivaldi hashes by Git SHA1 hash
func (rm *RefsManager) LookupByGitHash(gitSHA1 string) (blake3Hash [32]byte, sha256Hash [32]byte, err error) {
	blake3Hex, sha256Hex, err := rm.db.LookupByGitHash(gitSHA1)
	if err != nil {
		return [32]byte{}, [32]byte{}, err
	}
	
	blake3Bytes, err := hex.DecodeString(blake3Hex)
	if err != nil {
		return [32]byte{}, [32]byte{}, fmt.Errorf("decode blake3 hex: %w", err)
	}
	
	sha256Bytes, err := hex.DecodeString(sha256Hex)
	if err != nil {
		return [32]byte{}, [32]byte{}, fmt.Errorf("decode sha256 hex: %w", err)
	}
	
	copy(blake3Hash[:], blake3Bytes)
	copy(sha256Hash[:], sha256Bytes)
	
	return blake3Hash, sha256Hash, nil
}

// writeTimeline writes a timeline to disk
func (rm *RefsManager) writeTimeline(timeline Timeline) error {
	refPath := rm.getRefPath(timeline.Name, timeline.Type)
	
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(refPath), 0755); err != nil {
		return fmt.Errorf("create ref parent dir: %w", err)
	}

	// Format: blake3_hex sha256_hex git_sha1_hex timestamp description
	content := fmt.Sprintf("%s %s %s %d %s\n",
		hex.EncodeToString(timeline.Blake3Hash[:]),
		hex.EncodeToString(timeline.SHA256Hash[:]),
		timeline.GitSHA1Hash,
		timeline.LastUpdated.Unix(),
		timeline.Description,
	)

	return os.WriteFile(refPath, []byte(content), 0644)
}

// getRefPath returns the file path for a timeline reference
func (rm *RefsManager) getRefPath(name string, timelineType TimelineType) string {
	subdir := rm.getSubdir(timelineType)
	// Replace forward slashes with path separators for nested refs like "origin/main"
	safeName := strings.ReplaceAll(name, "/", string(filepath.Separator))
	return filepath.Join(rm.refsDir, subdir, safeName)
}

// getSubdir returns the subdirectory for a timeline type
func (rm *RefsManager) getSubdir(timelineType TimelineType) string {
	switch timelineType {
	case LocalTimeline:
		return "heads"
	case RemoteTimeline:
		return "remotes"
	case TagTimeline:
		return "tags"
	default:
		return "heads"
	}
}

// InitializeFromGit initializes refs from an existing Git repository
func (rm *RefsManager) InitializeFromGit(gitDir string) error {
	gitRefsDir := filepath.Join(gitDir, "refs")
	if _, err := os.Stat(gitRefsDir); os.IsNotExist(err) {
		return nil // No Git refs to import
	}

	// Import local branches
	headsDir := filepath.Join(gitRefsDir, "heads")
	if _, err := os.Stat(headsDir); err == nil {
		err := filepath.Walk(headsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}

			relPath, err := filepath.Rel(headsDir, path)
			if err != nil {
				return err
			}

			branchName := strings.ReplaceAll(relPath, string(filepath.Separator), "/")
			return rm.importGitRef(path, branchName, LocalTimeline)
		})
		if err != nil {
			return fmt.Errorf("import local branches: %w", err)
		}
	}

	// Import remote branches
	remotesDir := filepath.Join(gitRefsDir, "remotes")
	if _, err := os.Stat(remotesDir); err == nil {
		err := filepath.Walk(remotesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}

			relPath, err := filepath.Rel(remotesDir, path)
			if err != nil {
				return err
			}

			remoteName := strings.ReplaceAll(relPath, string(filepath.Separator), "/")
			return rm.importGitRef(path, remoteName, RemoteTimeline)
		})
		if err != nil {
			return fmt.Errorf("import remote branches: %w", err)
		}
	}

	// Import tags
	tagsDir := filepath.Join(gitRefsDir, "tags")
	if _, err := os.Stat(tagsDir); err == nil {
		err := filepath.Walk(tagsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}

			relPath, err := filepath.Rel(tagsDir, path)
			if err != nil {
				return err
			}

			tagName := strings.ReplaceAll(relPath, string(filepath.Separator), "/")
			return rm.importGitRef(path, tagName, TagTimeline)
		})
		if err != nil {
			return fmt.Errorf("import tags: %w", err)
		}
	}

	// Import HEAD
	headPath := filepath.Join(gitDir, "HEAD")
	if _, err := os.Stat(headPath); err == nil {
		headData, err := os.ReadFile(headPath)
		if err == nil {
			ivaldiHeadPath := filepath.Join(rm.ivaldiDir, "HEAD")
			os.WriteFile(ivaldiHeadPath, headData, 0644)
		}
	}

	return nil
}

// importGitRef imports a single Git reference
func (rm *RefsManager) importGitRef(gitRefPath, name string, timelineType TimelineType) error {
	data, err := os.ReadFile(gitRefPath)
	if err != nil {
		return err
	}

	gitSHA1 := strings.TrimSpace(string(data))
	if len(gitSHA1) != 40 {
		return fmt.Errorf("invalid Git SHA1 hash: %s", gitSHA1)
	}

	// For now, create placeholder hashes - in a real implementation,
	// we would look up the corresponding Ivaldi hashes or convert the Git object
	var blake3Hash, sha256Hash [32]byte
	
	// Try to find existing mapping (this would be implemented with proper Git->Ivaldi conversion)
	// For now, use zero hashes as placeholders
	
	description := fmt.Sprintf("Imported from Git ref: %s", gitSHA1)
	return rm.CreateTimeline(name, timelineType, blake3Hash, sha256Hash, gitSHA1, description)
}