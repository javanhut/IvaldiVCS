// Package resolution implements storage and management of conflict resolution decisions.
//
// Unlike Git which writes conflict markers directly into workspace files,
// Ivaldi stores resolution decisions separately, keeping the workspace clean.
// Resolution decisions can be reviewed, replayed, and used as templates.
package diffmerge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
)

// ResolutionChoice represents how a conflict was resolved.
type ResolutionChoice string

const (
	ChoiceOurs   ResolutionChoice = "ours"   // Kept target timeline version
	ChoiceTheirs ResolutionChoice = "theirs" // Accepted source timeline version
	ChoiceBase   ResolutionChoice = "base"   // Reverted to common ancestor
	ChoiceUnion  ResolutionChoice = "union"  // Combined both versions
	ChoiceCustom ResolutionChoice = "custom" // Manual/custom resolution
)

// ChunkResolution represents the resolution decision for a single chunk conflict.
type ChunkResolution struct {
	ChunkIndex int              `json:"chunk_index"`
	Choice     ResolutionChoice `json:"choice"`
	ResultHash *string          `json:"result_hash,omitempty"` // Hash of resolved chunk (for custom)
}

// FileResolution represents all resolution decisions for a single file.
type FileResolution struct {
	Path        string            `json:"path"`
	Strategy    string            `json:"strategy"`     // Strategy used (auto, ours, theirs, etc.)
	Resolved    bool              `json:"resolved"`     // Whether fully resolved
	Chunks      []ChunkResolution `json:"chunks"`       // Per-chunk resolutions
	ResultHash  string            `json:"result_hash"`  // Hash of final merged file
	ResolvedAt  time.Time         `json:"resolved_at"`  // When resolution was made
	ResolvedBy  string            `json:"resolved_by"`  // Who resolved it
}

// MergeResolution represents all resolution decisions for a merge operation.
type MergeResolution struct {
	SourceTimeline string                     `json:"source_timeline"`
	TargetTimeline string                     `json:"target_timeline"`
	SourceHash     string                     `json:"source_hash"`
	TargetHash     string                     `json:"target_hash"`
	Strategy       StrategyType               `json:"strategy"`       // Overall strategy
	Files          map[string]*FileResolution `json:"files"`          // Per-file resolutions
	CreatedAt      time.Time                  `json:"created_at"`
	CompletedAt    *time.Time                 `json:"completed_at,omitempty"`
	Status         string                     `json:"status"` // "in_progress", "resolved", "aborted"
}

// ResolutionStorage manages storage of merge resolution decisions.
type ResolutionStorage struct {
	ivaldiDir string
}

// NewResolutionStorage creates a new ResolutionStorage.
func NewResolutionStorage(ivaldiDir string) *ResolutionStorage {
	return &ResolutionStorage{
		ivaldiDir: ivaldiDir,
	}
}

// Save saves a merge resolution to disk.
func (rs *ResolutionStorage) Save(resolution *MergeResolution) error {
	resolutionPath := filepath.Join(rs.ivaldiDir, "MERGE_RESOLUTION")

	data, err := json.MarshalIndent(resolution, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resolution: %w", err)
	}

	err = os.WriteFile(resolutionPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write resolution: %w", err)
	}

	return nil
}

// Load loads a merge resolution from disk.
func (rs *ResolutionStorage) Load() (*MergeResolution, error) {
	resolutionPath := filepath.Join(rs.ivaldiDir, "MERGE_RESOLUTION")

	data, err := os.ReadFile(resolutionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No resolution in progress
		}
		return nil, fmt.Errorf("failed to read resolution: %w", err)
	}

	var resolution MergeResolution
	err = json.Unmarshal(data, &resolution)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal resolution: %w", err)
	}

	return &resolution, nil
}

// Delete removes the merge resolution from disk.
func (rs *ResolutionStorage) Delete() error {
	resolutionPath := filepath.Join(rs.ivaldiDir, "MERGE_RESOLUTION")
	err := os.Remove(resolutionPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete resolution: %w", err)
	}
	return nil
}

// Exists checks if a merge resolution exists.
func (rs *ResolutionStorage) Exists() bool {
	resolutionPath := filepath.Join(rs.ivaldiDir, "MERGE_RESOLUTION")
	_, err := os.Stat(resolutionPath)
	return err == nil
}

// SaveHistory archives a completed resolution for future reference.
func (rs *ResolutionStorage) SaveHistory(resolution *MergeResolution) error {
	historyDir := filepath.Join(rs.ivaldiDir, "merge-history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Create filename based on timestamp and timelines
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s_%s-to-%s.json", timestamp, resolution.SourceTimeline, resolution.TargetTimeline)
	historyPath := filepath.Join(historyDir, filename)

	data, err := json.MarshalIndent(resolution, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resolution: %w", err)
	}

	err = os.WriteFile(historyPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write history: %w", err)
	}

	return nil
}

// CreateResolution initializes a new merge resolution.
func CreateResolution(sourceTimeline, targetTimeline string, sourceHash, targetHash cas.Hash, strategy StrategyType) *MergeResolution {
	return &MergeResolution{
		SourceTimeline: sourceTimeline,
		TargetTimeline: targetTimeline,
		SourceHash:     sourceHash.String(),
		TargetHash:     targetHash.String(),
		Strategy:       strategy,
		Files:          make(map[string]*FileResolution),
		CreatedAt:      time.Now(),
		Status:         "in_progress",
	}
}

// AddFileResolution adds a file resolution to the merge resolution.
func (mr *MergeResolution) AddFileResolution(path string, result *ChunkMergeResult, strategy string, resolvedBy string) {
	resolution := &FileResolution{
		Path:       path,
		Strategy:   strategy,
		Resolved:   result.Success,
		ResolvedAt: time.Now(),
		ResolvedBy: resolvedBy,
	}

	// Add chunk resolutions if there were conflicts
	if !result.Success {
		for _, conflict := range result.Conflicts {
			chunkRes := ChunkResolution{
				ChunkIndex: conflict.ChunkIndex,
				Choice:     ChoiceCustom, // Will be updated when actually resolved
			}
			resolution.Chunks = append(resolution.Chunks, chunkRes)
		}
	}

	// Store result hash if available
	if len(result.MergedChunks) > 0 {
		// Use first chunk hash as representative (simplified)
		resolution.ResultHash = result.MergedChunks[0].String()
	}

	mr.Files[path] = resolution
}

// MarkCompleted marks the resolution as completed.
func (mr *MergeResolution) MarkCompleted() {
	now := time.Now()
	mr.CompletedAt = &now
	mr.Status = "resolved"
}

// MarkAborted marks the resolution as aborted.
func (mr *MergeResolution) MarkAborted() {
	now := time.Now()
	mr.CompletedAt = &now
	mr.Status = "aborted"
}

// GetUnresolvedFiles returns all files that still have conflicts.
func (mr *MergeResolution) GetUnresolvedFiles() []string {
	var unresolved []string
	for path, resolution := range mr.Files {
		if !resolution.Resolved {
			unresolved = append(unresolved, path)
		}
	}
	return unresolved
}

// IsFullyResolved checks if all files have been resolved.
func (mr *MergeResolution) IsFullyResolved() bool {
	for _, resolution := range mr.Files {
		if !resolution.Resolved {
			return false
		}
	}
	return true
}

// GetConflictCount returns the total number of unresolved conflicts.
func (mr *MergeResolution) GetConflictCount() int {
	count := 0
	for _, resolution := range mr.Files {
		if !resolution.Resolved {
			count += len(resolution.Chunks)
			if len(resolution.Chunks) == 0 {
				count++ // File-level conflict
			}
		}
	}
	return count
}

// Summary returns a human-readable summary of the resolution.
func (mr *MergeResolution) Summary() string {
	totalFiles := len(mr.Files)
	unresolvedFiles := len(mr.GetUnresolvedFiles())
	conflictCount := mr.GetConflictCount()

	if mr.IsFullyResolved() {
		return fmt.Sprintf("All %d files resolved using %s strategy", totalFiles, mr.Strategy)
	}

	return fmt.Sprintf("%d/%d files resolved, %d conflicts remaining",
		totalFiles-unresolvedFiles, totalFiles, conflictCount)
}
