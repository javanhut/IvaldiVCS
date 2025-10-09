package cli

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log [options]",
	Short: "Show commit history",
	Long: `Display the commit history for the current timeline.

Examples:
  ivaldi log                  # Show all commits
  ivaldi log --oneline        # Show concise one-line format
  ivaldi log --limit 10       # Show only last 10 commits
  ivaldi log --all            # Show commits from all timelines`,
	RunE: runLog,
}

var (
	logOneline bool
	logLimit   int
	logAll     bool
)

func init() {
	logCmd.Flags().BoolVar(&logOneline, "oneline", false, "Show one line per commit")
	logCmd.Flags().IntVar(&logLimit, "limit", 0, "Limit number of commits to show")
	logCmd.Flags().BoolVar(&logAll, "all", false, "Show commits from all timelines")
}

type commitInfo struct {
	Hash     cas.Hash
	Commit   *commit.CommitObject
	SealName string
	Timeline string
}

func runLog(cmd *cobra.Command, args []string) error {
	// Check if we're in an Ivaldi repository
	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	// Initialize refs manager
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs manager: %w", err)
	}
	defer refsManager.Close()

	// Initialize CAS
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	var commits []commitInfo

	if logAll {
		// Get commits from all timelines
		timelines, err := refsManager.ListTimelines(refs.LocalTimeline)
		if err != nil {
			return fmt.Errorf("failed to list timelines: %w", err)
		}

		for _, timeline := range timelines {
			timelineCommits, err := getTimelineCommits(casStore, refsManager, timeline.Name, timeline.Blake3Hash)
			if err != nil {
				continue // Skip timelines with errors
			}
			commits = append(commits, timelineCommits...)
		}

		// Sort commits by time (newest first)
		sortCommitsByTime(commits)
	} else {
		// Get current timeline
		currentTimeline, err := refsManager.GetCurrentTimeline()
		if err != nil {
			return fmt.Errorf("failed to get current timeline: %w", err)
		}

		// Get timeline info
		timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
		if err != nil {
			return fmt.Errorf("failed to get timeline info: %w", err)
		}

		// Get commits for current timeline
		commits, err = getTimelineCommits(casStore, refsManager, currentTimeline, timeline.Blake3Hash)
		if err != nil {
			return fmt.Errorf("failed to get commits: %w", err)
		}
	}

	if len(commits) == 0 {
		fmt.Println("No commits yet.")
		return nil
	}

	// Apply limit if specified
	if logLimit > 0 && len(commits) > logLimit {
		commits = commits[:logLimit]
	}

	// Display commits
	if logOneline {
		displayCommitsOneline(commits)
	} else {
		displayCommitsFull(commits)
	}

	return nil
}

// getTimelineCommits retrieves all commits for a timeline starting from HEAD
func getTimelineCommits(casStore cas.CAS, refsManager *refs.RefsManager, timelineName string, headHash [32]byte) ([]commitInfo, error) {
	if headHash == [32]byte{} {
		return nil, nil // No commits yet
	}

	var commits []commitInfo
	visited := make(map[cas.Hash]bool)

	commitReader := commit.NewCommitReader(casStore)

	// Start from HEAD and walk back through parents
	var currentHash cas.Hash
	copy(currentHash[:], headHash[:])

	for {
		// Avoid cycles
		if visited[currentHash] {
			break
		}
		visited[currentHash] = true

		// Read commit
		commitObj, err := commitReader.ReadCommit(currentHash)
		if err != nil {
			break // Stop on error
		}

		// Get seal name if available
		var hashArray [32]byte
		copy(hashArray[:], currentHash[:])
		sealName, _ := refsManager.GetSealNameByHash(hashArray)

		commits = append(commits, commitInfo{
			Hash:     currentHash,
			Commit:   commitObj,
			SealName: sealName,
			Timeline: timelineName,
		})

		// Move to parent
		if len(commitObj.Parents) == 0 {
			break // No more parents
		}

		// Follow first parent (for linear history)
		currentHash = commitObj.Parents[0]
	}

	return commits, nil
}

// sortCommitsByTime sorts commits by commit time (newest first)
func sortCommitsByTime(commits []commitInfo) {
	// Simple bubble sort since we don't expect huge commit lists
	for i := 0; i < len(commits); i++ {
		for j := i + 1; j < len(commits); j++ {
			if commits[i].Commit.CommitTime.Before(commits[j].Commit.CommitTime) {
				commits[i], commits[j] = commits[j], commits[i]
			}
		}
	}
}

// displayCommitsFull displays commits in full format
func displayCommitsFull(commits []commitInfo) {
	for i, info := range commits {
		// Seal name or short hash
		if info.SealName != "" {
			fmt.Printf("%s %s\n", colors.Cyan("seal"), colors.Bold(info.SealName))
		} else {
			shortHash := hex.EncodeToString(info.Hash[:4])
			fmt.Printf("%s %s\n", colors.Cyan("commit"), colors.Bold(shortHash))
		}

		// Author
		fmt.Printf("Author: %s\n", colors.InfoText(info.Commit.Author))

		// Date
		relTime := getRelativeTime(info.Commit.CommitTime)
		fmt.Printf("Date:   %s (%s)\n",
			info.Commit.CommitTime.Format("Mon Jan 2 15:04:05 2006"),
			colors.Gray(relTime))

		// Timeline (if showing all)
		if logAll {
			fmt.Printf("Timeline: %s\n", colors.InfoText(info.Timeline))
		}

		// Message
		fmt.Printf("\n    %s\n", info.Commit.Message)

		// Separator
		if i < len(commits)-1 {
			fmt.Println()
		}
	}
}

// displayCommitsOneline displays commits in one-line format
func displayCommitsOneline(commits []commitInfo) {
	for _, info := range commits {
		// Hash or seal name
		var id string
		if info.SealName != "" {
			id = colors.Cyan(info.SealName[:20]) // Truncate long names
		} else {
			shortHash := hex.EncodeToString(info.Hash[:4])
			id = colors.Cyan(shortHash)
		}

		// Message (first line only)
		message := info.Commit.Message
		if len(message) > 60 {
			message = message[:57] + "..."
		}

		// Timeline indicator (if showing all)
		timeline := ""
		if logAll {
			timeline = colors.Gray(fmt.Sprintf(" [%s]", info.Timeline))
		}

		fmt.Printf("%s %s%s\n", id, message, timeline)
	}
}

// getRelativeTime returns a human-readable relative time string
func getRelativeTime(t time.Time) string {
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
