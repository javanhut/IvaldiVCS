package engine

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
)

// CommitEntry holds display-ready information about a single commit
type CommitEntry struct {
	Hash      string    // Short hex hash
	FullHash  [32]byte  // Full hash for lookups
	SealName  string    // Seal name (if assigned)
	Author    string    // Author name
	Message   string    // Commit message
	Time      time.Time // Commit time
	Timeline  string    // Timeline this commit belongs to
	Parents   []string  // Parent short hashes
	IsMerge   bool      // Whether this is a merge commit
}

// LogOptions controls what commits to retrieve
type LogOptions struct {
	Limit       int  // Max commits to return (0 = unlimited)
	AllTimelines bool // Show commits from all timelines
}

// GetCommitHistory retrieves commit history for the repository
func GetCommitHistory(ivaldiDir string, opts LogOptions) ([]CommitEntry, string, error) {
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize storage: %w", err)
	}

	currentTimeline, err := refsManager.GetCurrentTimeline()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get current timeline: %w", err)
	}

	var commits []CommitEntry

	if opts.AllTimelines {
		timelines, err := refsManager.ListTimelines(refs.LocalTimeline)
		if err != nil {
			return nil, currentTimeline, fmt.Errorf("failed to list timelines: %w", err)
		}

		for _, timeline := range timelines {
			entries, err := walkTimeline(casStore, refsManager, timeline.Name, timeline.Blake3Hash)
			if err != nil {
				continue
			}
			commits = append(commits, entries...)
		}

		sortEntriesByTime(commits)
	} else {
		timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
		if err != nil {
			return nil, currentTimeline, fmt.Errorf("failed to get timeline info: %w", err)
		}

		commits, err = walkTimeline(casStore, refsManager, currentTimeline, timeline.Blake3Hash)
		if err != nil {
			return nil, currentTimeline, fmt.Errorf("failed to get commits: %w", err)
		}
	}

	if opts.Limit > 0 && len(commits) > opts.Limit {
		commits = commits[:opts.Limit]
	}

	return commits, currentTimeline, nil
}

// walkTimeline retrieves all commits for a timeline starting from HEAD
func walkTimeline(casStore cas.CAS, refsManager *refs.RefsManager, timelineName string, headHash [32]byte) ([]CommitEntry, error) {
	if headHash == [32]byte{} {
		return nil, nil
	}

	var entries []CommitEntry
	visited := make(map[cas.Hash]bool)
	commitReader := commit.NewCommitReader(casStore)

	var currentHash cas.Hash
	copy(currentHash[:], headHash[:])

	for {
		if visited[currentHash] {
			break
		}
		visited[currentHash] = true

		commitObj, err := commitReader.ReadCommit(currentHash)
		if err != nil {
			break
		}

		var hashArray [32]byte
		copy(hashArray[:], currentHash[:])
		sealName, _ := refsManager.GetSealNameByHash(hashArray)

		shortHash := hex.EncodeToString(currentHash[:4])

		var parentHashes []string
		for _, p := range commitObj.Parents {
			parentHashes = append(parentHashes, hex.EncodeToString(p[:4]))
		}

		entries = append(entries, CommitEntry{
			Hash:     shortHash,
			FullHash: hashArray,
			SealName: sealName,
			Author:   commitObj.Author,
			Message:  commitObj.Message,
			Time:     commitObj.CommitTime,
			Timeline: timelineName,
			Parents:  parentHashes,
			IsMerge:  len(commitObj.Parents) > 1,
		})

		if len(commitObj.Parents) == 0 {
			break
		}

		currentHash = commitObj.Parents[0]
	}

	return entries, nil
}

// sortEntriesByTime sorts commit entries by time (newest first)
func sortEntriesByTime(entries []CommitEntry) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Time.Before(entries[j].Time) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

// RelativeTime returns a human-readable relative time string
func RelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	if diff < 30*24*time.Hour {
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}
	if diff < 365*24*time.Hour {
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	years := int(diff.Hours() / 24 / 365)
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}
