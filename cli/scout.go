package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/github"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var scoutCmd = &cobra.Command{
	Use:   "scout",
	Short: "Discover remote timelines available for harvest",
	Long: `Scout discovers remote timelines (branches) available on the GitHub repository
that haven't been harvested locally yet. This helps you see what new branches
are available before deciding to harvest them.

Examples:
  ivaldi scout                    # Discover all remote timelines
  ivaldi scout --refresh          # Refresh remote timeline cache`,
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

		// Get GitHub repository configuration
		owner, repo, err := refsManager.GetGitHubRepository()
		if err != nil {
			return fmt.Errorf("no GitHub repository configured. Use 'ivaldi portal add owner/repo' or download from GitHub first")
		}

		// Create syncer
		syncer, err := github.NewRepoSyncer(ivaldiDir, workDir)
		if err != nil {
			return fmt.Errorf("failed to create GitHub syncer: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		fmt.Printf("Scouting GitHub repository: %s/%s\n\n", owner, repo)

		// Get remote timelines (this also updates the refs)
		remoteBranches, err := syncer.GetRemoteTimelines(ctx, owner, repo)
		if err != nil {
			return fmt.Errorf("failed to get remote timelines: %w", err)
		}

		// Get current timeline
		currentTimeline, err := refsManager.GetCurrentTimeline()
		if err != nil {
			currentTimeline = "(unknown)"
		}

		// Get sync statuses
		syncStatuses, err := refsManager.GetTimelineSyncStatuses()
		if err != nil {
			return fmt.Errorf("failed to get timeline sync statuses: %w", err)
		}

		// Sort for consistent output
		sort.Slice(syncStatuses, func(i, j int) bool {
			return syncStatuses[i].Name < syncStatuses[j].Name
		})

		// Display results
		remoteOnlyCount := 0
		localOnlyCount := 0
		bothCount := 0

		fmt.Println("Remote Timelines:")
		for _, status := range syncStatuses {
			if status.Status == "remote-only" {
				fmt.Printf("  %-20s (new, ready to harvest)\n", status.Name)
				remoteOnlyCount++
			}
		}

		fmt.Println("\nLocal Timelines:")
		for _, status := range syncStatuses {
			if status.LocalExists {
				if status.RemoteExists {
					fmt.Printf("  %-20s (exists locally and remotely)\n", status.Name)
					bothCount++
				} else {
					fmt.Printf("  %-20s (local only, not on remote)\n", status.Name)
					localOnlyCount++
				}
			}
		}

		fmt.Println("\nSummary:")
		fmt.Printf("  • Current timeline: %s\n", currentTimeline)
		fmt.Printf("  • Remote timelines available to harvest: %d\n", remoteOnlyCount)
		fmt.Printf("  • Timelines that exist both locally and remotely: %d\n", bothCount)
		fmt.Printf("  • Local-only timelines: %d\n", localOnlyCount)
		fmt.Printf("  • Total remote timelines discovered: %d\n", len(remoteBranches))

		// Helpful next steps
		if remoteOnlyCount > 0 {
			fmt.Println("\nNext steps:")
			fmt.Println("  • Use 'ivaldi harvest <timeline-name>' to download specific timelines")
			fmt.Println("  • Use 'ivaldi harvest' to download all new timelines")
		} else {
			fmt.Println("\nAll remote timelines are already available locally!")
		}

		return nil
	},
}

var scoutRefreshFlag bool

func init() {
	scoutCmd.Flags().BoolVar(&scoutRefreshFlag, "refresh", false, "Refresh remote timeline information")
}
