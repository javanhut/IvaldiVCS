package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/javanhut/Ivaldi-vcs/internal/converter"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ivaldi",
	Short: "Ivaldi is a Version Control System",
	Long:  `Ivaldi is a VCS used to control repo that can be used to replace Git in your normal workflow`,
}

var initialCmd = &cobra.Command{
	Use:   "forge",
	Short: "Initialize",
	Long:  "Initializes a new ivaldi managed repository",
	Run:   forgeCommand,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Core commands
	rootCmd.AddCommand(initialCmd)

	// Timeline management commands
	rootCmd.AddCommand(timelineCmd)
	timelineCmd.AddCommand(createTimelineCmd, switchTimelineCmd, listTimelineCmd, removeTimelineCmd)

	// File and commit management commands
	rootCmd.AddCommand(gatherCmd)
	rootCmd.AddCommand(sealCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(excludeCommand)

	// Remote repository commands (now with GitHub integration)
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(downloadCmd)

	// Portal commands for repository connection management
	rootCmd.AddCommand(portalCmd)
	portalCmd.AddCommand(portalAddCmd, portalListCmd, portalRemoveCmd)
}

func forgeCommand(cmd *cobra.Command, args []string) {
	numOfArgs := len(args)
	if numOfArgs > 0 {
		errMsg := fmt.Sprintf("Forge takes in 0 argument %d was given.", numOfArgs)
		log.Fatal(errMsg)
	}

	ivaldiDir := ".ivaldi"
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Get working directory: %v", err)
	}

	// Create Ivaldi directory
	err = os.Mkdir(ivaldiDir, os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}

	log.Println("Ivaldi repository initialized")

	// Initialize refs system
	log.Println("Initializing timeline management system...")
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize refs system: %v", err)
	} else {
		defer refsManager.Close()

		// Check if we're in a Git repository
		if _, err := os.Stat(".git"); err == nil {
			log.Println("Detecting existing Git repository, importing refs and converting objects...")

			// Import Git refs first
			if err := refsManager.InitializeFromGit(".git"); err != nil {
				log.Printf("Warning: Failed to import Git refs: %v", err)
			} else {
				log.Println("Successfully imported Git refs to Ivaldi timeline system")
			}

			// Convert Git objects with shared database connection using concurrent workers
			log.Println("Converting Git objects to Ivaldi format...")
			gitResult, err := converter.ConvertGitObjectsToIvaldiConcurrent(".git", ivaldiDir, 8)
			if err != nil {
				log.Printf("Warning: Failed to convert Git objects: %v", err)
			} else {
				log.Printf("Successfully converted %d Git objects", gitResult.Converted)
				if gitResult.Skipped > 0 {
					log.Printf("Skipped %d Git objects due to errors", gitResult.Skipped)
				}
			}
		} else {
			// Initialize default timeline for new repository
			log.Println("Creating default 'main' timeline...")

			// Initially create main timeline with zero hashes
			var zeroHash [32]byte
			err = refsManager.CreateTimeline(
				"main",
				refs.LocalTimeline,
				zeroHash, // blake3Hash
				zeroHash, // sha256Hash
				"",       // gitSHA1Hash
				"Initial empty repository",
			)
			if err != nil {
				log.Printf("Warning: Failed to create main timeline: %v", err)
			} else {
				log.Println("Successfully created main timeline")
			}

			// Set main as current timeline
			if err := refsManager.SetCurrentTimeline("main"); err != nil {
				log.Printf("Warning: Failed to set current timeline: %v", err)
			}
		}
	}

	// Create snapshot of current files using concurrent workers
	log.Println("Creating snapshot of current files...")
	result, err := converter.SnapshotCurrentFilesConcurrent(workDir, ivaldiDir, 8)
	if err != nil {
		log.Printf("Warning: Failed to snapshot files: %v", err)
	} else {
		log.Printf("Snapshotted %d files as blob objects", result.Converted)
		if result.Skipped > 0 {
			log.Printf("Skipped %d files due to errors", result.Skipped)
		}
		if len(result.Errors) > 0 {
			log.Printf("Errors encountered during snapshot:")
			for _, e := range result.Errors[:min(3, len(result.Errors))] { // Show first 3 errors
				log.Printf("  - %v", e)
			}
			if len(result.Errors) > 3 {
				log.Printf("  ... and %d more errors", len(result.Errors)-3)
			}
		}

		// If we snapshotted files, create an initial commit
		if result.Converted > 0 {
			log.Println("Creating initial commit for existing files...")
			commitHash, err := createInitialCommit(ivaldiDir, workDir)
			if err != nil {
				log.Printf("Warning: Failed to create initial commit: %v", err)
			} else if commitHash != nil {
				// Update main timeline to point to the initial commit
				log.Println("Updating main timeline with initial commit...")

				// Re-open refs manager to update the timeline
				refsManager2, err := refs.NewRefsManager(ivaldiDir)
				if err != nil {
					log.Printf("Warning: Failed to reopen refs manager: %v", err)
				} else {
					defer refsManager2.Close()

					// Update main timeline with the commit hash
					err = refsManager2.UpdateTimeline(
						"main",
						refs.LocalTimeline,
						*commitHash, // Use the actual commit hash
						[32]byte{},  // No SHA256 for now
						"",          // No Git SHA1
					)
					if err != nil {
						log.Printf("Warning: Failed to update main timeline with initial commit: %v", err)
					} else {
						log.Println("Successfully updated main timeline with initial commit")
					}
				}
			}
		}
	}

	// Create initial snapshot for status tracking
	log.Println("Creating initial snapshot for status tracking...")
	if err := updateLastSnapshot(workDir, ivaldiDir); err != nil {
		log.Printf("Warning: Failed to create initial snapshot: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
