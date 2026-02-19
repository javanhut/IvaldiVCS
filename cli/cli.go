package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/javanhut/Ivaldi-vcs/internal/converter"
	"github.com/javanhut/Ivaldi-vcs/internal/logging"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

const IvaldiVersion = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "ivaldi",
	Short: "Ivaldi is a Version Control System",
	Long:  `Ivaldi is a VCS used to control repo that can be used to replace Git in your normal workflow`,
	Run: func(cmd *cobra.Command, args []string) {
		if version {
			fmt.Printf("Ivaldi Version %s\n", IvaldiVersion)
			os.Exit(0)
		}
		// If no version flag, show help
		cmd.Help()
	},
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

var (
	version bool
	verbose int  // -v count for verbosity
	quiet   bool // -q flag for quiet mode
)

func init() {
	// Initialize logging on startup
	cobra.OnInitialize(initLogging)

	// Global flags (available to all commands)
	rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "Increase output verbosity (-v for info, -vv for debug)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")

	// Core commands
	rootCmd.Flags().BoolVar(&version, "version", false, "Use this to get the Version of Ivaldi")
	rootCmd.AddCommand(initialCmd)

	// Timeline management commands
	rootCmd.AddCommand(timelineCmd)
	timelineCmd.AddCommand(createTimelineCmd, switchTimelineCmd, listTimelineCmd, removeTimelineCmd, renameTimelineCmd)
	timelineCmd.AddCommand(butterflyCmd)
	butterflyCmd.AddCommand(butterflyUpCmd, butterflyDownCmd, butterflyRemoveCmd)

	// File and commit management commands
	rootCmd.AddCommand(gatherCmd)
	rootCmd.AddCommand(sealCmd)
	rootCmd.AddCommand(sealsCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(whereamiCmd)
	rootCmd.AddCommand(excludeCommand)

	// Remote repository commands (now with GitHub integration)
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(downloadCmd)

	// Portal commands for repository connection management
	rootCmd.AddCommand(portalCmd)
	portalCmd.AddCommand(portalAddCmd, portalListCmd, portalRemoveCmd)

	// Remote timeline discovery and harvesting commands
	rootCmd.AddCommand(scoutCmd)
	rootCmd.AddCommand(harvestCmd)

	// Configuration command
	rootCmd.AddCommand(configCmd)

	// History and comparison commands
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(resetCmd)

	// Merge command
	rootCmd.AddCommand(fuseCmd)

	// Time travel command
	rootCmd.AddCommand(travelCmd)

	// Shift command (commit squashing)
	rootCmd.AddCommand(shiftCmd)

	// Sync command
	rootCmd.AddCommand(syncCmd)
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

	logging.Info("Ivaldi repository initialized")

	// Initialize refs system
	logging.Info("Initializing timeline management system...")
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		logging.Warn("Failed to initialize refs system", "error", err)
	} else {
		defer refsManager.Close()

		// Check if we're in a Git repository
		if _, err := os.Stat(".git"); err == nil {
			logging.Info("Detecting existing Git repository, importing refs and converting objects...")

			// Import Git refs first
			if err := refsManager.InitializeFromGit(".git"); err != nil {
				logging.Warn("Failed to import Git refs", "error", err)
			} else {
				logging.Info("Successfully imported Git refs to Ivaldi timeline system")
			}

			// Convert Git objects with shared database connection using concurrent workers
			logging.Info("Converting Git objects to Ivaldi format...")
			gitResult, err := converter.ConvertGitObjectsToIvaldiConcurrent(".git", ivaldiDir, 16)
			if err != nil {
				logging.Warn("Failed to convert Git objects", "error", err)
			} else {
				logging.Info("Successfully converted Git objects", "count", gitResult.Converted)
				if gitResult.Skipped > 0 {
					logging.Warn("Skipped Git objects due to errors", "count", gitResult.Skipped)
				}
			}

			if _, err := os.Stat(".gitmodules"); err == nil {
				logging.Info("Detected Git submodules, converting to Ivaldi format...")

				submoduleResult, err := converter.ConvertGitSubmodulesToIvaldi(
					".git",
					ivaldiDir,
					workDir,
					true,
				)

				if err != nil {
					logging.Warn("Submodule conversion encountered errors", "error", err)
				}

				if submoduleResult.Converted > 0 {
					logging.Info("Converted Git submodules", "count", submoduleResult.Converted)
				}
				if submoduleResult.ClonedModules > 0 {
					logging.Info("Cloned missing submodules", "count", submoduleResult.ClonedModules)
				}
				if submoduleResult.Skipped > 0 {
					logging.Warn("Skipped submodules due to errors", "count", submoduleResult.Skipped)
					for i, err := range submoduleResult.Errors {
						if i < 3 {
							logging.Warn("Submodule error", "error", err)
						}
					}
					if len(submoduleResult.Errors) > 3 {
						logging.Warn("Additional submodule errors", "count", len(submoduleResult.Errors)-3)
					}
				}
			}
		} else {
			// Initialize default timeline for new repository
			logging.Info("Creating default 'main' timeline...")

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
				logging.Warn("Failed to create main timeline", "error", err)
			} else {
				logging.Info("Successfully created main timeline")
			}

			// Set main as current timeline
			if err := refsManager.SetCurrentTimeline("main"); err != nil {
				logging.Warn("Failed to set current timeline", "error", err)
			}
		}
	}

	// Create snapshot of current files using concurrent workers
	logging.Info("Creating snapshot of current files...")
	result, err := converter.SnapshotCurrentFilesConcurrent(workDir, ivaldiDir, 8)
	if err != nil {
		logging.Warn("Failed to snapshot files", "error", err)
	} else {
		logging.Info("Snapshotted files as blob objects", "count", result.Converted)
		if result.Skipped > 0 {
			logging.Warn("Skipped files due to errors", "count", result.Skipped)
		}
		if len(result.Errors) > 0 {
			logging.Warn("Errors encountered during snapshot", "count", len(result.Errors))
			for _, e := range result.Errors[:min(3, len(result.Errors))] { // Show first 3 errors
				logging.Warn("Snapshot error", "error", e)
			}
			if len(result.Errors) > 3 {
				logging.Warn("Additional snapshot errors", "count", len(result.Errors)-3)
			}
		}

		// If we snapshotted files, create an initial commit
		if result.Converted > 0 {
			logging.Info("Creating initial commit for existing files...")
			commitHash, err := createInitialCommit(ivaldiDir, workDir)
			if err != nil {
				logging.Warn("Failed to create initial commit", "error", err)
			} else if commitHash != nil {
				// Update main timeline to point to the initial commit
				logging.Info("Updating main timeline with initial commit...")

				// Re-open refs manager to update the timeline
				refsManager2, err := refs.NewRefsManager(ivaldiDir)
				if err != nil {
					logging.Warn("Failed to reopen refs manager", "error", err)
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
						logging.Warn("Failed to update main timeline with initial commit", "error", err)
					} else {
						logging.Info("Successfully updated main timeline with initial commit")
					}
				}
			}
		}
	}

	// Create initial snapshot for status tracking
	logging.Info("Creating initial snapshot for status tracking...")
	if err := updateLastSnapshot(workDir, ivaldiDir); err != nil {
		logging.Warn("Failed to create initial snapshot", "error", err)
	}
}

// initLogging initializes the logging system based on CLI flags.
func initLogging() {
	var level logging.Level
	if quiet {
		level = logging.LevelQuiet
	} else {
		level = logging.Level(verbose)
	}
	logging.Init(level)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
