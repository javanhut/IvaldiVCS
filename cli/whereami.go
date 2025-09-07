package cli

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/diffmerge"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/spf13/cobra"
)

var whereamiCmd = &cobra.Command{
	Use:     "whereami",
	Aliases: []string{"wai"},
	Short:   "Show current timeline (branch) information",
	Long: `Display detailed information about the current timeline including:
- Timeline name and type
- Last commit information
- Remote sync status
- Brief workspace status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we're in an Ivaldi repository
		ivaldiDir := ".ivaldi"
		if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
			return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
		}

		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		// Initialize refs manager
		refsManager, err := refs.NewRefsManager(ivaldiDir)
		if err != nil {
			return fmt.Errorf("failed to initialize refs manager: %w", err)
		}
		defer refsManager.Close()

		// Get current timeline
		currentTimelineName, err := refsManager.GetCurrentTimeline()
		if err != nil {
			return fmt.Errorf("failed to get current timeline: %w", err)
		}

		// Get timeline details
		timeline, err := refsManager.GetTimeline(currentTimelineName, refs.LocalTimeline)
		if err != nil {
			return fmt.Errorf("failed to get timeline details: %w", err)
		}

		// Display basic timeline info
		fmt.Printf("Timeline: %s\n", colors.Bold(currentTimelineName))
		fmt.Printf("Type: %s\n", colors.InfoText("Local Timeline"))

		// Get last commit information if timeline has commits
		if timeline.Blake3Hash != [32]byte{} {
			err = displayCommitInfo(ivaldiDir, timeline, refsManager)
			if err != nil {
				fmt.Printf("Last Seal: %s (unable to read commit details: %v)\n",
					hex.EncodeToString(timeline.Blake3Hash[:])[:8], err)
			}
		} else {
			fmt.Printf("Last Seal: (no seals yet)\n")
		}

		// Check remote sync status
		err = displayRemoteStatus(refsManager, currentTimelineName)
		if err != nil {
			// Don't fail the command if remote status check fails
			fmt.Printf("Remote: (unable to check remote status: %v)\n", err)
		}

		// Display workspace status summary
		err = displayWorkspaceStatus(ivaldiDir, workDir)
		if err != nil {
			fmt.Printf("Workspace: (unable to check workspace status: %v)\n", err)
		}

		fmt.Println()
		return nil
	},
}

// displayCommitInfo shows information about the last commit (now called seal)
func displayCommitInfo(ivaldiDir string, timeline *refs.Timeline, refsManager *refs.RefsManager) error {
	// Try to get seal name first
	sealName, err := refsManager.GetSealNameByHash(timeline.Blake3Hash)

	// Initialize CAS to read commit
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err2 := cas.NewFileCAS(objectsDir)
	if err2 != nil {
		return fmt.Errorf("failed to initialize CAS: %w", err2)
	}

	// Convert timeline hash to CAS hash
	var commitHash cas.Hash
	copy(commitHash[:], timeline.Blake3Hash[:])

	// Read commit object
	commitReader := commit.NewCommitReader(casStore)
	commitObj, err2 := commitReader.ReadCommit(commitHash)
	if err2 != nil {
		return fmt.Errorf("failed to read commit: %w", err2)
	}

	// Format commit info with seal name or hash fallback
	timeAgo := formatTimeAgo(commitObj.CommitTime)

	if err == nil {
		// Use seal name if available
		fmt.Printf("Last Seal: %s (%s)\n", colors.Cyan(sealName), colors.Dim(timeAgo))
	} else {
		// Fall back to hash if no seal name
		shortHash := hex.EncodeToString(timeline.Blake3Hash[:])[:8]
		fmt.Printf("Last Seal: %s (%s)\n", colors.Cyan(shortHash), colors.Dim(timeAgo))
	}

	// Show commit message (first line only)
	message := strings.Split(strings.TrimSpace(commitObj.Message), "\n")[0]
	if len(message) > 60 {
		message = message[:57] + "..."
	}
	fmt.Printf("Message: \"%s\"\n", message)

	return nil
}

// displayRemoteStatus shows sync status with remote timeline
func displayRemoteStatus(refsManager *refs.RefsManager, timelineName string) error {
	// Check if there's a GitHub repository configured
	owner, repo, err := refsManager.GetGitHubRepository()
	if err != nil {
		fmt.Printf("Remote: (no GitHub repository configured)\n")
		return nil
	}

	// Check if there's a corresponding remote timeline
	remoteName := fmt.Sprintf("origin/%s", timelineName)
	remoteTimeline, err := refsManager.GetTimeline(remoteName, refs.RemoteTimeline)
	if err != nil {
		fmt.Printf("Remote: %s/%s (not tracked)\n", owner, repo)
		return nil
	}

	// Get local timeline for comparison
	localTimeline, err := refsManager.GetTimeline(timelineName, refs.LocalTimeline)
	if err != nil {
		return fmt.Errorf("failed to get local timeline: %w", err)
	}

	// Compare hashes to determine sync status
	var status string
	if localTimeline.Blake3Hash == remoteTimeline.Blake3Hash {
		status = "up to date"
	} else {
		status = "needs comparison"
	}

	fmt.Printf("Remote: %s/%s (%s)\n", owner, repo, status)
	return nil
}

// displayWorkspaceStatus shows a summary of workspace changes
func displayWorkspaceStatus(ivaldiDir, workDir string) error {
	// Initialize CAS for workspace scanning
	objectsDir := filepath.Join(ivaldiDir, "objects")
	casStore, err := cas.NewFileCAS(objectsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize CAS: %w", err)
	}

	// Create materializer to get workspace status
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
	status, err := materializer.GetWorkspaceStatus()
	if err != nil {
		return fmt.Errorf("failed to get workspace status: %w", err)
	}

	if status.Clean {
		fmt.Printf("Workspace: %s\n", colors.SuccessText("Clean"))
	} else {
		var added, modified, deleted int
		for _, change := range status.Changes {
			switch change.Type {
			case diffmerge.Added:
				added++
			case diffmerge.Modified:
				modified++
			case diffmerge.Removed:
				deleted++
			}
		}

		parts := []string{}
		if added > 0 {
			parts = append(parts, colors.Green(fmt.Sprintf("%d added", added)))
		}
		if modified > 0 {
			parts = append(parts, colors.Blue(fmt.Sprintf("%d modified", modified)))
		}
		if deleted > 0 {
			parts = append(parts, colors.Red(fmt.Sprintf("%d deleted", deleted)))
		}

		fmt.Printf("Workspace: %s\n", strings.Join(parts, ", "))
	}

	// Check for staged files
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	if _, err := os.Stat(stageFile); err == nil {
		stageData, err := os.ReadFile(stageFile)
		if err == nil {
			stagedFiles := strings.Fields(string(stageData))
			if len(stagedFiles) > 0 {
				fmt.Printf("Staged: %s files ready for seal\n", colors.Green(fmt.Sprintf("%d", len(stagedFiles))))
			}
		}
	}

	return nil
}

// formatTimeAgo formats a time as "X ago" string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if duration < 30*24*time.Hour {
		weeks := int(duration.Hours() / (7 * 24))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / (30 * 24))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	} else {
		years := int(duration.Hours() / (365 * 24))
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}
