package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/seals"
	"github.com/javanhut/Ivaldi-vcs/internal/shelf"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
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
		currentTimeline, _ := refsManager.GetCurrentTimeline()
		var baseHashes [2][32]byte // blake3 and sha256 hashes
		var casStore cas.CAS

		// Initialize CAS
		objectsDir := filepath.Join(ivaldiDir, "objects")
		casStore, err = cas.NewFileCAS(objectsDir)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		if currentTimeline == "" {
			// No current timeline, create from scratch with zero hashes
			log.Printf("No current timeline found, creating new timeline from scratch")
		} else {
			log.Printf("Creating timeline '%s' branched from '%s'", name, currentTimeline)

			// FIRST: Create an auto-shelf for the CURRENT timeline to preserve its untracked files
			// This ensures files like tl1.txt stay with tl1 when we create tl2
			shelfManager := shelf.NewShelfManager(casStore, ivaldiDir)
			materializer := workspace.NewMaterializer(casStore, ivaldiDir, ".")
			currentWorkspaceIndex, err := materializer.ScanWorkspace()
			if err == nil {
				// Get the current timeline's base (committed) state
				var currentBaseIndex wsindex.IndexRef
				currentTimelineRef, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
				if err == nil && currentTimelineRef.Blake3Hash != [32]byte{} {
					// Timeline has commits, get its committed state
					wsBuilder := wsindex.NewBuilder(casStore)
					currentBaseIndex, _ = wsBuilder.Build(nil) // Simplified - should read actual commit
				} else {
					// No commits, use empty base
					wsBuilder := wsindex.NewBuilder(casStore)
					currentBaseIndex, _ = wsBuilder.Build(nil)
				}

				// Create auto-shelf for current timeline BEFORE creating new timeline
				_, err = shelfManager.CreateAutoShelf(currentTimeline, currentWorkspaceIndex, currentBaseIndex)
				if err != nil {
					log.Printf("Warning: Failed to auto-shelf current timeline: %v", err)
				} else {
					log.Printf("Auto-shelved workspace state for timeline '%s'", currentTimeline)
				}
			}

			// THEN: Capture the workspace state for the NEW timeline
			log.Printf("Capturing current workspace state for new timeline")
			err = createCommitFromWorkspace(casStore, ivaldiDir, currentTimeline, &baseHashes)
			if err != nil {
				log.Printf("Warning: Could not create workspace snapshot: %v", err)

				// Fall back to parent timeline's committed state if snapshot fails
				timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
				if err == nil && timeline.Blake3Hash != [32]byte{} {
					baseHashes[0] = timeline.Blake3Hash
					baseHashes[1] = timeline.SHA256Hash
					log.Printf("Falling back to parent timeline's committed state")
				}
			} else {
				log.Printf("Created commit from current workspace snapshot")
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

		// If we created from a parent timeline, switch to the new timeline
		if currentTimeline != "" && baseHashes[0] != [32]byte{} {
			// Simply update the current timeline reference
			// The workspace already has the correct files for the new timeline
			// (we captured them in the commit above)
			err = refsManager.SetCurrentTimeline(name)
			if err != nil {
				return fmt.Errorf("failed to set current timeline: %w", err)
			}

			fmt.Printf("Switched to new timeline '%s'\n", name)
			fmt.Printf("Timeline '%s' inherited workspace from '%s'\n", name, currentTimeline)
		}

		return nil
	},
}

var listTimelineCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all timelines",
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
	Use:     "switch <name>",
	Aliases: []string{"sw"},
	Short:   "Switch to a timeline (auto-shelving if needed)",
	Args:    cobra.ExactArgs(1),
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

		// Check for uncommitted changes
		objectsDir := filepath.Join(ivaldiDir, "objects")
		casStore, err := cas.NewFileCAS(objectsDir)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}

		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)

		// Materialize the target timeline with auto-shelving enabled
		// This will automatically stash uncommitted changes and restore any existing shelf
		err = materializer.MaterializeTimelineWithAutoShelf(name, true)
		if err != nil {
			return fmt.Errorf("failed to materialize timeline '%s': %w", name, err)
		}

		fmt.Printf("Switched to timeline '%s'\n", name)
		fmt.Printf("Workspace files updated to match timeline state.\n")

		return nil
	},
}

var removeTimelineCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a timeline",
	Args:    cobra.ExactArgs(1),
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
			return fmt.Errorf("cannot remove current timeline '%s'. Switch to another timeline first", name)
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

// createCommitFromWorkspace creates a commit object from the current workspace state
// and stores the commit hash in the provided baseHashes array.
func createCommitFromWorkspace(casStore cas.CAS, ivaldiDir string, parentTimeline string, baseHashes *[2][32]byte) error {
	// Get the parent timeline's commit if it exists
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to create refs manager: %w", err)
	}
	defer refsManager.Close()

	var parentCommitHash cas.Hash
	var workspaceFiles []wsindex.FileMetadata

	// Get parent timeline's commit hash for linking
	if parentTimeline != "" {
		timeline, err := refsManager.GetTimeline(parentTimeline, refs.LocalTimeline)
		if err == nil && timeline.Blake3Hash != [32]byte{} {
			copy(parentCommitHash[:], timeline.Blake3Hash[:])
		}
	}

	// Scan current workspace to capture ALL files (both tracked and untracked)
	// This becomes the initial state of the new timeline
	materializer := workspace.NewMaterializer(casStore, ivaldiDir, ".")
	wsIndex, err := materializer.ScanWorkspace()
	if err != nil {
		return fmt.Errorf("failed to scan workspace: %w", err)
	}

	wsLoader := wsindex.NewLoader(casStore)
	workspaceFiles, err = wsLoader.ListAll(wsIndex)
	if err != nil {
		return fmt.Errorf("failed to list workspace files: %w", err)
	}

	// Initialize persistent MMR for commit tracking
	mmr, err := history.NewPersistentMMR(casStore, ivaldiDir)
	if err != nil {
		// Fall back to in-memory MMR if persistent fails
		log.Printf("Warning: Failed to create persistent MMR, using in-memory: %v", err)
		mmr = &history.PersistentMMR{MMR: history.NewMMR()}
	}
	defer mmr.Close()

	// Create commit builder
	commitBuilder := commit.NewCommitBuilder(casStore, mmr.MMR)

	// Set parent for the commit if we have one
	var parents []cas.Hash
	if parentCommitHash != (cas.Hash{}) {
		parents = append(parents, parentCommitHash)
	}

	// Create commit from workspace files with proper parent linkage
	commitObj, err := commitBuilder.CreateCommit(
		workspaceFiles,
		parents,
		"ivaldi-system", // author
		"ivaldi-system", // committer
		fmt.Sprintf("Timeline branch from %s", parentTimeline),
	)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	// Get the commit hash - CreateCommit already stored the commit in CAS
	commitHash := commitBuilder.GetCommitHash(commitObj)

	// Set the base hashes to point to this commit
	copy(baseHashes[0][:], commitHash[:])
	// Leave baseHashes[1] as zero for Blake3-only commits

	// Generate and store seal name for this commit
	var commitHashArray [32]byte
	copy(commitHashArray[:], commitHash[:])
	sealName := seals.GenerateSealName(commitHashArray)
	err = refsManager.StoreSealName(sealName, commitHashArray, fmt.Sprintf("Timeline branch from %s", parentTimeline))
	if err != nil {
		log.Printf("Warning: Failed to store seal name: %v", err)
	}

	return nil
}
