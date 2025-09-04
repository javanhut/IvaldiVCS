package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var timelineCmd = &cobra.Command{
	Use:     "timeline",
	Aliases: []string{"tl"},
	Short:   "Manage Ivaldi Timelines",
	Long:    `Create, List, Switch, Remove Timelines`,
}

var createTimelineCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new timeline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		
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
		
		// Get current timeline to branch from
		currentTimeline, err := refsManager.GetCurrentTimeline()
		var baseHashes [2][32]byte // blake3 and sha256 hashes
		
		if err != nil {
			// No current timeline, create from scratch with zero hashes
			log.Printf("No current timeline found, creating new timeline from scratch")
		} else {
			// Get current timeline's hashes to branch from
			timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
			if err != nil {
				log.Printf("Warning: Could not get current timeline hashes, creating from scratch: %v", err)
			} else {
				baseHashes[0] = timeline.Blake3Hash
				baseHashes[1] = timeline.SHA256Hash
				log.Printf("Creating timeline '%s' branched from '%s'", name, currentTimeline)
			}
		}
		
		// Create the new timeline
		err = refsManager.CreateTimeline(
			name,
			refs.LocalTimeline,
			baseHashes[0], // blake3Hash
			baseHashes[1], // sha256Hash
			"",            // gitSHA1Hash (empty for new timeline)
			fmt.Sprintf("Created timeline '%s'", name),
		)
		if err != nil {
			return fmt.Errorf("failed to create timeline: %w", err)
		}
		
		fmt.Printf("Successfully created timeline: %s\n", name)
		return nil
	},
}

var listTimelineCmd = &cobra.Command{
	Use:   "list",
	Short: "List all timelines",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		
		// Get current timeline
		currentTimeline, err := refsManager.GetCurrentTimeline()
		if err != nil {
			log.Printf("Warning: Could not determine current timeline: %v", err)
		}
		
		// List local timelines
		localTimelines, err := refsManager.ListTimelines(refs.LocalTimeline)
		if err != nil {
			return fmt.Errorf("failed to list local timelines: %w", err)
		}
		
		// List remote timelines
		remoteTimelines, err := refsManager.ListTimelines(refs.RemoteTimeline)
		if err != nil {
			log.Printf("Warning: Failed to list remote timelines: %v", err)
		}
		
		// List tags
		tags, err := refsManager.ListTimelines(refs.TagTimeline)
		if err != nil {
			log.Printf("Warning: Failed to list tags: %v", err)
		}
		
		// Display results
		if len(localTimelines) > 0 {
			fmt.Println("Local Timelines:")
			for _, timeline := range localTimelines {
				marker := "  "
				if currentTimeline == timeline.Name {
					marker = "* " // Mark current timeline
				}
				fmt.Printf("%s%s\t%s\n", marker, timeline.Name, timeline.Description)
			}
		} else {
			fmt.Println("No local timelines found.")
		}
		
		if len(remoteTimelines) > 0 {
			fmt.Println("\nRemote Timelines:")
			for _, timeline := range remoteTimelines {
				fmt.Printf("  %s\t%s\n", timeline.Name, timeline.Description)
			}
		}
		
		if len(tags) > 0 {
			fmt.Println("\nTags:")
			for _, timeline := range tags {
				fmt.Printf("  %s\t%s\n", timeline.Name, timeline.Description)
			}
		}
		
		return nil
	},
}

var switchTimelineCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch to a timeline (auto-shelving if needed)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		
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
		
		// Check if the target timeline exists
		_, err = refsManager.GetTimeline(name, refs.LocalTimeline)
		if err != nil {
			return fmt.Errorf("timeline '%s' does not exist: %w", name, err)
		}
		
		// Get current timeline
		currentTimeline, err := refsManager.GetCurrentTimeline()
		if err == nil && currentTimeline == name {
			fmt.Printf("Already on timeline '%s'\n", name)
			return nil
		}
		
		// TODO: In a full implementation, we would:
		// 1. Check for uncommitted changes
		// 2. Auto-shelve changes if needed
		// 3. Update working directory to match target timeline state
		
		// For now, just update the HEAD reference
		err = refsManager.SetCurrentTimeline(name)
		if err != nil {
			return fmt.Errorf("failed to switch to timeline '%s': %w", name, err)
		}
		
		fmt.Printf("Switched to timeline '%s'\n", name)
		
		// Note: In a full VCS implementation, we would also need to:
		// - Update working directory files to match the timeline's state
		// - Handle merge conflicts if there are uncommitted changes
		// - Implement auto-shelving functionality
		fmt.Println("Note: Working directory files are not updated in this implementation.")
		fmt.Println("In a full VCS, files would be updated to match the timeline state.")
		
		return nil
	},
}

var removeTimelineCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a timeline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		
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
		
		// Check if timeline exists
		_, err = refsManager.GetTimeline(name, refs.LocalTimeline)
		if err != nil {
			return fmt.Errorf("timeline '%s' does not exist: %w", name, err)
		}
		
		// Check if trying to remove current timeline
		currentTimeline, err := refsManager.GetCurrentTimeline()
		if err == nil && currentTimeline == name {
			return fmt.Errorf("cannot remove current timeline '%s'. Switch to another timeline first.", name)
		}
		
		// Remove timeline file
		refPath := fmt.Sprintf("%s/refs/heads/%s", ivaldiDir, name)
		err = os.Remove(refPath)
		if err != nil {
			return fmt.Errorf("failed to remove timeline file: %w", err)
		}
		
		fmt.Printf("Successfully removed timeline '%s'\n", name)
		
		// Note: In a full implementation, we might want to:
		// - Check if timeline has unmerged commits
		// - Offer to create a backup
		// - Clean up orphaned objects if this was the only reference
		
		return nil
	},
}
