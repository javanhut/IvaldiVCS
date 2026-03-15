package engine

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/github"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

// PortalInfo holds the configured remote repository connection
type PortalInfo struct {
	Owner    string
	Repo     string
	Timeline string
	HasAuth  bool
}

// ScoutResult holds the results of scouting remote timelines
type ScoutResult struct {
	RemoteOnly []string
	LocalOnly  []string
	Both       []string
	Total      int
}

// SyncResult holds the results of a sync operation
type SyncResult struct {
	Timeline  string
	Added     []string
	Modified  []string
	Deleted   []string
	NoChanges bool
}

// UploadResult holds the results of an upload operation
type UploadResult struct {
	Timeline string
	Branch   string
	Owner    string
	Repo     string
}

// HarvestResult holds the results of a harvest operation
type HarvestResult struct {
	Successful []string
	Failed     []HarvestFailure
}

// HarvestFailure records a single timeline harvest failure
type HarvestFailure struct {
	Name string
	Err  string
}

// GetPortalInfo returns the configured remote repository connection
func GetPortalInfo(ivaldiDir string) (*PortalInfo, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	owner, repo, err := refsManager.GetGitHubRepository()
	if err != nil {
		return nil, fmt.Errorf("no repository configured")
	}

	currentTimeline, _ := refsManager.GetCurrentTimeline()

	// Check auth availability
	syncer, sErr := github.NewRepoSyncerOptionalAuth(ivaldiDir, ".")
	hasAuth := sErr == nil && syncer != nil && syncer.IsAuthenticated()

	return &PortalInfo{
		Owner:    owner,
		Repo:     repo,
		Timeline: currentTimeline,
		HasAuth:  hasAuth,
	}, nil
}

// SetPortal configures the remote repository connection
func SetPortal(ivaldiDir, ownerRepo string) error {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	// Parse owner/repo
	for i, c := range ownerRepo {
		if c == '/' {
			owner := ownerRepo[:i]
			repo := ownerRepo[i+1:]
			if owner == "" || repo == "" {
				return fmt.Errorf("invalid format. Use: owner/repo")
			}
			return refsManager.SetGitHubRepository(owner, repo)
		}
	}
	return fmt.Errorf("invalid format. Use: owner/repo")
}

// RemovePortal removes the remote repository connection
func RemovePortal(ivaldiDir string) error {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	return refsManager.RemoveGitHubRepository()
}

// Scout discovers remote timelines and their sync status
func Scout(ivaldiDir, workDir string) (*ScoutResult, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	owner, repo, err := refsManager.GetGitHubRepository()
	if err != nil {
		return nil, fmt.Errorf("no repository configured. Use portal to add one")
	}

	syncer, err := github.NewRepoSyncerOptionalAuth(ivaldiDir, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create syncer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	_, err = syncer.GetRemoteTimelines(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote timelines: %w", err)
	}

	syncStatuses, err := refsManager.GetTimelineSyncStatuses()
	if err != nil {
		return nil, fmt.Errorf("failed to get sync statuses: %w", err)
	}

	sort.Slice(syncStatuses, func(i, j int) bool {
		return syncStatuses[i].Name < syncStatuses[j].Name
	})

	result := &ScoutResult{}
	for _, status := range syncStatuses {
		switch {
		case status.Status == "remote-only":
			result.RemoteOnly = append(result.RemoteOnly, status.Name)
		case status.LocalExists && !status.RemoteExists:
			result.LocalOnly = append(result.LocalOnly, status.Name)
		case status.LocalExists && status.RemoteExists:
			result.Both = append(result.Both, status.Name)
		}
	}
	result.Total = len(syncStatuses)

	return result, nil
}

// Upload pushes the current timeline to the remote repository
func Upload(ivaldiDir, workDir string, force bool) (*UploadResult, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return nil, fmt.Errorf("failed to get current timeline: %w", err)
	}

	owner, repo, err := refsManager.GetGitHubRepository()
	if err != nil {
		return nil, fmt.Errorf("no repository configured")
	}

	timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline info: %w", err)
	}

	if timeline.Blake3Hash == [32]byte{} {
		return nil, fmt.Errorf("no commits to push")
	}

	var commitHash cas.Hash
	copy(commitHash[:], timeline.Blake3Hash[:])

	syncer, err := github.NewRepoSyncer(ivaldiDir, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create syncer (authentication required): %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := syncer.PushCommit(ctx, owner, repo, currentTimeline, commitHash, force, currentTimeline); err != nil {
		return nil, fmt.Errorf("failed to push: %w", err)
	}

	return &UploadResult{
		Timeline: currentTimeline,
		Branch:   currentTimeline,
		Owner:    owner,
		Repo:     repo,
	}, nil
}

// SyncTimeline syncs the current or specified timeline with remote
func SyncTimeline(ivaldiDir, workDir, timelineName string) (*SyncResult, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	if timelineName == "" {
		timelineName, err = refsManager.GetCurrentTimeline()
		if err != nil {
			return nil, fmt.Errorf("failed to get current timeline: %w", err)
		}
	}

	owner, repo, err := refsManager.GetGitHubRepository()
	if err != nil {
		return nil, fmt.Errorf("no repository configured")
	}

	timeline, err := refsManager.GetTimeline(timelineName, refs.LocalTimeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline '%s': %w", timelineName, err)
	}

	syncer, err := github.NewRepoSyncer(ivaldiDir, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create syncer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	delta, err := syncer.SyncTimeline(ctx, owner, repo, timelineName, timeline.Blake3Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sync: %w", err)
	}

	return &SyncResult{
		Timeline:  timelineName,
		Added:     delta.AddedFiles,
		Modified:  delta.ModifiedFiles,
		Deleted:   delta.DeletedFiles,
		NoChanges: delta.NoChanges,
	}, nil
}

// HarvestTimelines downloads remote timelines into the local repository
func HarvestTimelines(ivaldiDir, workDir string, names []string) (*HarvestResult, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	owner, repo, err := refsManager.GetGitHubRepository()
	if err != nil {
		return nil, fmt.Errorf("no repository configured")
	}

	syncer, err := github.NewRepoSyncerOptionalAuth(ivaldiDir, workDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create syncer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Refresh remote timelines
	_, err = syncer.GetRemoteTimelines(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to discover remote timelines: %w", err)
	}

	// If no names specified, harvest all remote-only
	if len(names) == 0 {
		syncStatuses, err := refsManager.GetTimelineSyncStatuses()
		if err != nil {
			return nil, fmt.Errorf("failed to get sync statuses: %w", err)
		}
		for _, status := range syncStatuses {
			if status.Status == "remote-only" {
				names = append(names, status.Name)
			}
		}
		sort.Strings(names)
	}

	if len(names) == 0 {
		return &HarvestResult{}, nil
	}

	result := &HarvestResult{}
	for _, name := range names {
		err := syncer.FetchTimeline(ctx, owner, repo, name)
		if err != nil {
			result.Failed = append(result.Failed, HarvestFailure{
				Name: name,
				Err:  err.Error(),
			})
		} else {
			result.Successful = append(result.Successful, name)
		}
	}

	return result, nil
}
