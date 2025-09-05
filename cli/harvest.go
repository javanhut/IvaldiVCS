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

var harvestCmd = &cobra.Command{
	Use:   "harvest [timeline-name...]",
	Short: "Harvest remote timelines into local repository",
	Long: `Harvest downloads remote timelines (branches) from GitHub into your local
Ivaldi repository. You can harvest specific timelines or all new ones.

After harvesting, you can switch to the new timeline using 'ivaldi timeline switch'.

Examples:
  ivaldi harvest                          # Harvest all new remote timelines
  ivaldi harvest feature-branch           # Harvest specific timeline
  ivaldi harvest main feature-x bugfix    # Harvest multiple specific timelines
  ivaldi harvest --update                 # Also update existing timelines`,
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

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		fmt.Printf("Harvesting from GitHub repository: %s/%s\n", owner, repo)

		// Get remote timelines to refresh our knowledge
		fmt.Println("Discovering remote timelines...")
		_, err = syncer.GetRemoteTimelines(ctx, owner, repo)
		if err != nil {
			return fmt.Errorf("failed to discover remote timelines: %w", err)
		}

		// Determine which timelines to harvest
		var timelinesToHarvest []string

		if len(args) == 0 {
			// Harvest all new timelines
			syncStatuses, err := refsManager.GetTimelineSyncStatuses()
			if err != nil {
				return fmt.Errorf("failed to get timeline sync statuses: %w", err)
			}

			for _, status := range syncStatuses {
				if status.Status == "remote-only" {
					timelinesToHarvest = append(timelinesToHarvest, status.Name)
				} else if harvestUpdateFlag && status.Status == "needs-comparison" {
					timelinesToHarvest = append(timelinesToHarvest, status.Name)
				}
			}

			if len(timelinesToHarvest) == 0 {
				if harvestUpdateFlag {
					fmt.Println("No new or updated timelines to harvest.")
				} else {
					fmt.Println("No new timelines to harvest.")
					fmt.Println("Use 'ivaldi harvest --update' to also update existing timelines.")
				}
				return nil
			}

			sort.Strings(timelinesToHarvest)
		} else {
			// Harvest specific timelines
			timelinesToHarvest = args
		}

		// Harvest each timeline
		successful := 0
		failed := 0

		for _, timelineName := range timelinesToHarvest {
			fmt.Printf("\n📦 Harvesting timeline: %s\n", timelineName)

			// Check if timeline exists locally (for update case)
			localExists := refsManager.TimelineExists(timelineName, refs.LocalTimeline)
			if localExists && !harvestUpdateFlag && len(args) == 0 {
				fmt.Printf("⚠️  Timeline '%s' already exists locally, skipping (use --update to force)\n", timelineName)
				continue
			}

			// Save current directory state before harvesting
			currentFiles := make(map[string]bool)
			if entries, err := os.ReadDir(workDir); err == nil {
				for _, entry := range entries {
					if entry.Name() != ".ivaldi" {
						currentFiles[entry.Name()] = true
					}
				}
			}

			err := syncer.FetchTimeline(ctx, owner, repo, timelineName)
			if err != nil {
				fmt.Printf("❌ Failed to harvest timeline '%s': %v\n", timelineName, err)
				failed++

				// Clean up any partial state
				if entries, err := os.ReadDir(workDir); err == nil {
					for _, entry := range entries {
						if entry.Name() != ".ivaldi" && !currentFiles[entry.Name()] {
							os.RemoveAll(entry.Name()) // Clean up any files that were created
						}
					}
				}
				continue
			}

			if localExists {
				fmt.Printf("✅ Updated timeline: %s\n", timelineName)
			} else {
				fmt.Printf("✅ Harvested new timeline: %s\n", timelineName)
			}
			successful++
		}

		// Summary
		fmt.Printf("\n📊 Harvest Summary:\n")
		fmt.Printf("  • Successfully harvested: %d timelines\n", successful)
		if failed > 0 {
			fmt.Printf("  • Failed to harvest: %d timelines\n", failed)
		}

		if successful > 0 {
			fmt.Printf("\n💡 Next steps:\n")
			if len(timelinesToHarvest) == 1 {
				fmt.Printf("  • Use 'ivaldi timeline switch %s' to switch to the harvested timeline\n", timelinesToHarvest[0])
			} else {
				fmt.Printf("  • Use 'ivaldi timeline list' to see all available timelines\n")
				fmt.Printf("  • Use 'ivaldi timeline switch <name>' to switch to a harvested timeline\n")
			}
		}

		// If we harvested successfully but there were failures, return an error code
		if successful > 0 && failed > 0 {
			fmt.Printf("\n⚠️  Some timelines failed to harvest. Check the errors above.\n")
		} else if failed > 0 {
			return fmt.Errorf("failed to harvest %d timeline(s)", failed)
		}

		return nil
	},
}

var harvestUpdateFlag bool

func init() {
	harvestCmd.Flags().BoolVar(&harvestUpdateFlag, "update", false, "Also update existing timelines with remote changes")
}
