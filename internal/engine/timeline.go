package engine

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/javanhut/Ivaldi-vcs/internal/butterfly"
	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/ignore"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
)

// TimelineInfo holds display information about a timeline
type TimelineInfo struct {
	Name        string
	IsCurrent   bool
	IsButterfly bool
	Hash        string // Short hex hash
	Description string
	Type        refs.TimelineType
}

// TimelineListResult holds all timelines grouped by type
type TimelineListResult struct {
	Current   string
	Local     []TimelineInfo
	Remote    []TimelineInfo
	Tags      []TimelineInfo
}

// ListTimelines returns all timelines grouped by type
func ListTimelines(ivaldiDir string) (*TimelineListResult, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	currentTimeline, _ := refsManager.GetCurrentTimeline()

	// Initialize butterfly manager
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, _ := cas.NewFileCAS(objectsDir)
	var bfManager *butterfly.Manager
	if casStore != nil {
		mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
		if err == nil {
			defer mmr.Close()
			bfManager, err = butterfly.NewManager(ivaldiDir, casStore, refsManager, mmr)
			if err == nil && bfManager != nil {
				defer bfManager.Close()
			}
		}
	}

	result := &TimelineListResult{
		Current: currentTimeline,
	}

	// Local timelines
	localTimelines, err := refsManager.ListTimelines(refs.LocalTimeline)
	if err != nil {
		return nil, fmt.Errorf("failed to list local timelines: %w", err)
	}
	for _, tl := range localTimelines {
		info := TimelineInfo{
			Name:        tl.Name,
			IsCurrent:   tl.Name == currentTimeline,
			Description: tl.Description,
			Type:        refs.LocalTimeline,
			Hash:        shortHash(tl.Blake3Hash),
		}
		if bfManager != nil {
			info.IsButterfly = bfManager.IsButterfly(tl.Name)
		}
		result.Local = append(result.Local, info)
	}

	// Remote timelines
	remoteTimelines, _ := refsManager.ListTimelines(refs.RemoteTimeline)
	for _, tl := range remoteTimelines {
		result.Remote = append(result.Remote, TimelineInfo{
			Name:        tl.Name,
			Description: tl.Description,
			Type:        refs.RemoteTimeline,
			Hash:        shortHash(tl.Blake3Hash),
		})
	}

	// Tags
	tags, _ := refsManager.ListTimelines(refs.TagTimeline)
	for _, tl := range tags {
		result.Tags = append(result.Tags, TimelineInfo{
			Name:        tl.Name,
			Description: tl.Description,
			Type:        refs.TagTimeline,
			Hash:        shortHash(tl.Blake3Hash),
		})
	}

	return result, nil
}

// SwitchTimeline switches to a different timeline with auto-shelving
func SwitchTimeline(ivaldiDir, workDir, name string) error {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	// Check timeline exists
	_, err = refsManager.GetTimeline(name, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("timeline '%s' does not exist: %w", name, err)
	}

	// Check if already on this timeline
	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err == nil && currentTimeline == name {
		return fmt.Errorf("already on timeline '%s'", name)
	}

	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
	ignoreCache, _ := ignore.LoadPatternCache(workDir)
	materializer.SetIgnorePatterns(ignoreCache)

	err = materializer.MaterializeTimelineWithAutoShelf(name, true)
	if err != nil {
		return fmt.Errorf("failed to switch to timeline '%s': %w", name, err)
	}

	return nil
}

// RemoveTimeline removes a timeline
func RemoveTimeline(ivaldiDir, name string) error {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	// Check it exists
	_, err = refsManager.GetTimeline(name, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("timeline '%s' does not exist: %w", name, err)
	}

	// Prevent removing current timeline
	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err == nil && currentTimeline == name {
		return fmt.Errorf("cannot remove current timeline '%s'. Switch to another timeline first", name)
	}

	refPath := fmt.Sprintf("%s/refs/heads/%s", ivaldiDir, name)
	if err := os.Remove(refPath); err != nil {
		return fmt.Errorf("failed to remove timeline file: %w", err)
	}

	return nil
}

// RenameTimeline renames a timeline
func RenameTimeline(ivaldiDir, oldName, newName string, force bool) error {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	return refsManager.RenameTimeline(oldName, newName, refs.LocalTimeline, force)
}

// CreateTimeline creates a new local timeline branched from the current one
func CreateTimeline(ivaldiDir, name string) error {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	// Get current timeline to branch from
	currentTimeline, _ := refsManager.GetCurrentTimeline()
	var blake3Hash, sha256Hash [32]byte

	if currentTimeline != "" {
		timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
		if err == nil && timeline.Blake3Hash != [32]byte{} {
			blake3Hash = timeline.Blake3Hash
			sha256Hash = timeline.SHA256Hash
		}
	}

	err = refsManager.CreateTimeline(
		name,
		refs.LocalTimeline,
		blake3Hash,
		sha256Hash,
		"",
		fmt.Sprintf("Created timeline '%s'", name),
	)
	if err != nil {
		return fmt.Errorf("failed to create timeline: %w", err)
	}

	return nil
}

// shortHash returns the first 8 hex characters of a hash, or empty if zero
func shortHash(hash [32]byte) string {
	if hash == [32]byte{} {
		return ""
	}
	return hex.EncodeToString(hash[:])[:8]
}
