package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/github"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local timeline with remote changes",
	Long: `Synchronize the current local timeline with remote repository changes.
Downloads only the delta of changes (added/removed files) from the remote
and updates the local timeline to match.

Examples:
  ivaldi sync                    # Sync current timeline with remote
  ivaldi sync main               # Sync specific timeline with remote`,
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

		// Determine which timeline to sync
		var timelineToSync string
		if len(args) > 0 {
			timelineToSync = args[0]
		} else {
			// Use current timeline
			currentTimeline, err := refsManager.GetCurrentTimeline()
			if err != nil {
				return fmt.Errorf("failed to get current timeline: %w", err)
			}
			timelineToSync = currentTimeline
		}

		// Get GitHub repository configuration
		owner, repo, err := refsManager.GetGitHubRepository()
		if err != nil {
			return fmt.Errorf("no GitHub repository configured. Use 'ivaldi portal add owner/repo' or download from GitHub first")
		}

		// Get local timeline state
		timeline, err := refsManager.GetTimeline(timelineToSync, refs.LocalTimeline)
		if err != nil {
			return fmt.Errorf("failed to get timeline '%s': %w", timelineToSync, err)
		}

		// Create syncer
		syncer, err := github.NewRepoSyncer(ivaldiDir, workDir)
		if err != nil {
			return fmt.Errorf("failed to create GitHub syncer: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		fmt.Printf("Syncing timeline '%s' with %s/%s...\n\n",
			colors.Bold(timelineToSync), owner, repo)

		// Perform sync and get delta information
		delta, err := syncer.SyncTimeline(ctx, owner, repo, timelineToSync, timeline.Blake3Hash)
		if err != nil {
			return fmt.Errorf("failed to sync timeline: %w", err)
		}

		// Display results
		if delta.NoChanges {
			fmt.Printf("%s Timeline '%s' is already up to date\n",
				colors.Green("✓"), colors.Bold(timelineToSync))
			return nil
		}

		// Sort files for consistent output
		sort.Strings(delta.AddedFiles)
		sort.Strings(delta.ModifiedFiles)
		sort.Strings(delta.DeletedFiles)

		// Display added files
		for _, file := range delta.AddedFiles {
			fmt.Printf("%s %s\n", colors.Green("++"), file)
		}

		// Display modified files (show as added in the output)
		for _, file := range delta.ModifiedFiles {
			fmt.Printf("%s %s\n", colors.Green("++"), file)
		}

		// Display deleted files
		for _, file := range delta.DeletedFiles {
			fmt.Printf("%s %s\n", colors.Red("--"), file)
		}

		// Summary
		totalChanges := len(delta.AddedFiles) + len(delta.ModifiedFiles) + len(delta.DeletedFiles)
		fmt.Printf("\n%s Synced %d file(s) from remote\n",
			colors.Green("✓"), totalChanges)

		if len(delta.AddedFiles) > 0 {
			fmt.Printf("  • Added: %s\n", colors.Green(fmt.Sprintf("%d", len(delta.AddedFiles))))
		}
		if len(delta.ModifiedFiles) > 0 {
			fmt.Printf("  • Modified: %s\n", colors.Blue(fmt.Sprintf("%d", len(delta.ModifiedFiles))))
		}
		if len(delta.DeletedFiles) > 0 {
			fmt.Printf("  • Deleted: %s\n", colors.Red(fmt.Sprintf("%d", len(delta.DeletedFiles))))
		}

		return nil
	},
}
