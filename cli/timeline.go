package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
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
		currentTimeline, err := refsManager.GetCurrentTimeline()
		var baseHashes [2][32]byte // blake3 and sha256 hashes

		if err != nil {
			// No current timeline, create from scratch with zero hashes
			log.Printf("No current timeline found, creating new timeline from scratch")
		} else {
			log.Printf("Creating timeline '%s' branched from '%s'", name, currentTimeline)

			// First try to get the parent timeline's committed state
			timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
			if err != nil {
				log.Printf("Warning: Could not get current timeline: %v", err)
			} else {
				// Use parent timeline's commit hash if it has one
				if timeline.Blake3Hash != [32]byte{} {
					baseHashes[0] = timeline.Blake3Hash
					baseHashes[1] = timeline.SHA256Hash
					log.Printf("Using parent timeline's committed state")
				} else {
					// Parent timeline is empty, check if workspace has files
					log.Printf("Parent timeline '%s' is empty, checking workspace for files", currentTimeline)

					// Initialize CAS and workspace materializer to snapshot current workspace
					objectsDir := filepath.Join(ivaldiDir, "objects")
					casStore, err := cas.NewFileCAS(objectsDir)
					if err != nil {
						return fmt.Errorf("failed to initialize storage: %w", err)
					}

					// Create snapshot of current workspace state
					err = createCommitFromWorkspace(casStore, ivaldiDir, currentTimeline, &baseHashes)
					if err != nil {
						log.Printf("Warning: Could not create workspace snapshot: %v", err)
						// Leave baseHashes as zero for empty timeline
					} else {
						log.Printf("Created commit from workspace snapshot")

						// IMPORTANT: Update the parent timeline with this commit!
						// This ensures the files belong to the parent timeline
						if baseHashes[0] != [32]byte{} {
							log.Printf("Updating parent timeline '%s' with workspace commit", currentTimeline)
							err = refsManager.UpdateTimeline(
								currentTimeline,
								refs.LocalTimeline,
								baseHashes[0], // blake3Hash
								baseHashes[1], // sha256Hash
								"",            // gitSHA1Hash
							)
							if err != nil {
								log.Printf("Warning: Failed to update parent timeline: %v", err)
							} else {
								log.Printf("Parent timeline '%s' updated with workspace files", currentTimeline)
							}
						}
					}
				}
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

		// If we created from a parent timeline, materialize the files
		if currentTimeline != "" && baseHashes[0] != [32]byte{} {
			// Switch to the new timeline to materialize its files
			fmt.Printf("Materializing files from parent timeline '%s'...\n", currentTimeline)

			// Initialize workspace materializer
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

			// Set the new timeline as current
			err = refsManager.SetCurrentTimeline(name)
			if err != nil {
				return fmt.Errorf("failed to set current timeline: %w", err)
			}

			// Materialize the timeline
			err = materializer.MaterializeTimelineWithAutoShelf(name, false) // No auto-shelf for new timeline
			if err != nil {
				log.Printf("Warning: Failed to materialize timeline files: %v", err)
				log.Printf("You may need to run 'ivaldi timeline switch %s' to materialize files", name)
			} else {
				fmt.Printf("Files materialized from parent timeline '%s'\n", currentTimeline)
			}
		}

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
		err = materializer.MaterializeTimeline(name)
		if err != nil {
			return fmt.Errorf("failed to materialize timeline '%s': %w", name, err)
		}

		fmt.Printf("Switched to timeline '%s'\n", name)
		fmt.Printf("Workspace files updated to match timeline state.\n")

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

// createCommitFromWorkspace creates a commit object that branches from the parent timeline
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

	if parentTimeline != "" {
		// Get parent timeline's commit
		timeline, err := refsManager.GetTimeline(parentTimeline, refs.LocalTimeline)
		if err == nil && timeline.Blake3Hash != [32]byte{} {
			copy(parentCommitHash[:], timeline.Blake3Hash[:])

			// Read the parent commit to get its tree
			commitReader := commit.NewCommitReader(casStore)
			parentCommit, err := commitReader.ReadCommit(parentCommitHash)
			if err != nil {
				return fmt.Errorf("failed to read parent commit: %w", err)
			}

			// Read the parent's tree to get all files
			tree, err := commitReader.ReadTree(parentCommit)
			if err != nil {
				return fmt.Errorf("failed to read parent tree: %w", err)
			}

			// List all files from the parent tree
			filePaths, err := commitReader.ListFiles(tree)
			if err != nil {
				return fmt.Errorf("failed to list parent files: %w", err)
			}

			// Create file metadata for each file from parent
			for _, filePath := range filePaths {
				content, err := commitReader.GetFileContent(tree, filePath)
				if err != nil {
					return fmt.Errorf("failed to get content for file %s: %w", filePath, err)
				}

				// Create file chunks for the content
				builder := filechunk.NewBuilder(casStore, filechunk.DefaultParams())
				fileRef, err := builder.Build(content)
				if err != nil {
					return fmt.Errorf("failed to create file chunks for %s: %w", filePath, err)
				}

				// Create file metadata from parent commit
				fileMetadata := wsindex.FileMetadata{
					Path:     filePath,
					FileRef:  fileRef,
					ModTime:  parentCommit.CommitTime,
					Mode:     0644, // Default file mode
					Size:     int64(len(content)),
					Checksum: cas.SumB3(content),
				}

				workspaceFiles = append(workspaceFiles, fileMetadata)
			}
		}
	}

	// If no parent files, scan current workspace
	if len(workspaceFiles) == 0 {
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

	return nil
}
