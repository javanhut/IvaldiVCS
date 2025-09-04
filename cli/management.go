package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
	"github.com/spf13/cobra"
)

var uploadCmd = &cobra.Command{
	Use:     "upload [remote] [timeline]",
	Aliases: []string{"push"},
	Short:   "Upload timeline to remote repository",
	Long:    `Uploads the current timeline or specified timeline to a remote repository`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we're in an Ivaldi repository
		ivaldiDir := ".ivaldi"
		if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
			return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
		}

		remote := "origin"
		if len(args) > 0 {
			remote = args[0]
		}

		// Initialize refs manager
		refsManager, err := refs.NewRefsManager(ivaldiDir)
		if err != nil {
			return fmt.Errorf("failed to initialize refs manager: %w", err)
		}
		defer refsManager.Close()

		// Get current timeline or use specified one
		timeline := ""
		if len(args) > 1 {
			timeline = args[1]
		} else {
			timeline, err = refsManager.GetCurrentTimeline()
			if err != nil {
				return fmt.Errorf("failed to get current timeline: %w", err)
			}
		}

		// TODO: Implement actual upload functionality
		// This would involve:
		// 1. Connecting to remote repository
		// 2. Determining what objects need to be uploaded
		// 3. Creating pack files of missing objects
		// 4. Uploading pack files and updating remote refs
		// 5. Handling authentication and protocols (HTTP, SSH, etc.)

		fmt.Printf("Uploading timeline '%s' to remote '%s'...\n", timeline, remote)
		fmt.Println("Note: Remote upload functionality not yet implemented.")
		fmt.Println("In a full implementation, this would upload objects and refs to a remote repository.")

		return nil
	},
}

var downloadCmd = &cobra.Command{
	Use:     "download <url> [directory]",
	Aliases: []string{"clone"},
	Short:   "Download/clone repository from remote",
	Long:    `Downloads a complete repository from a remote URL into a new directory`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		
		// Determine target directory
		targetDir := ""
		if len(args) > 1 {
			targetDir = args[1]
		} else {
			// Extract directory name from URL
			parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
			targetDir = strings.TrimSuffix(parts[len(parts)-1], ".git")
		}

		// Check if directory already exists
		if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
			return fmt.Errorf("directory '%s' already exists", targetDir)
		}

		// TODO: Implement actual download/clone functionality
		// This would involve:
		// 1. Creating target directory
		// 2. Initializing Ivaldi repository
		// 3. Connecting to remote repository
		// 4. Downloading object pack files
		// 5. Extracting and storing objects in local format
		// 6. Setting up remote references
		// 7. Checking out working directory to match HEAD

		fmt.Printf("Downloading repository from '%s' into '%s'...\n", url, targetDir)
		fmt.Println("Note: Remote download/clone functionality not yet implemented.")
		fmt.Println("In a full implementation, this would:")
		fmt.Println("  1. Create the target directory")
		fmt.Println("  2. Initialize Ivaldi repository")
		fmt.Println("  3. Download all objects and refs from remote")
		fmt.Println("  4. Set up working directory")

		return nil
	},
}

var gatherCmd = &cobra.Command{
	Use:   "gather [files...]",
	Short: "Stage files for the next seal/commit",
	Long:  `Gathers (stages) specified files or all modified files that will be included in the next seal operation`,
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

		// Create staging area directory
		stageDir := filepath.Join(ivaldiDir, "stage")
		if err := os.MkdirAll(stageDir, 0755); err != nil {
			return fmt.Errorf("failed to create staging directory: %w", err)
		}

		var filesToGather []string

		if len(args) == 0 {
			// If no arguments, gather all modified files
			fmt.Println("No files specified, gathering all files in working directory...")
			err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip directories and hidden files/dirs
				if info.IsDir() || filepath.Base(path)[0] == '.' {
					return nil
				}

				// Get relative path
				relPath, err := filepath.Rel(workDir, path)
				if err != nil {
					return err
				}

				// Skip .ivaldi directory
				if strings.HasPrefix(relPath, ".ivaldi") {
					return nil
				}

				filesToGather = append(filesToGather, relPath)
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to walk directory: %w", err)
			}
		} else {
			// Use specified files
			for _, file := range args {
				// Check if file exists
				if _, err := os.Stat(file); os.IsNotExist(err) {
					log.Printf("Warning: File '%s' does not exist, skipping", file)
					continue
				}
				filesToGather = append(filesToGather, file)
			}
		}

		if len(filesToGather) == 0 {
			fmt.Println("No files to gather.")
			return nil
		}

		// Write staged files list
		stageFile := filepath.Join(stageDir, "files")
		f, err := os.Create(stageFile)
		if err != nil {
			return fmt.Errorf("failed to create stage file: %w", err)
		}
		defer f.Close()

		for _, file := range filesToGather {
			if _, err := f.WriteString(file + "\n"); err != nil {
				return fmt.Errorf("failed to write to stage file: %w", err)
			}
			fmt.Printf("Gathered: %s\n", file)
		}

		fmt.Printf("Successfully gathered %d files for staging.\n", len(filesToGather))
		fmt.Println("Use 'ivaldi seal <message>' to create a commit with these files.")

		return nil
	},
}

var sealCmd = &cobra.Command{
	Use:   "seal <message>",
	Short: "Create a sealed commit with gathered files",
	Args:  cobra.ExactArgs(1),
	Long:  `Creates a sealed commit (equivalent to git commit) with the files that were gathered (staged)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		message := args[0]
		
		// Check if we're in an Ivaldi repository
		ivaldiDir := ".ivaldi"
		if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
			return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
		}

		// Check if there are staged files
		stageFile := filepath.Join(ivaldiDir, "stage", "files")
		if _, err := os.Stat(stageFile); os.IsNotExist(err) {
			return fmt.Errorf("no files staged for commit. Use 'ivaldi gather' to stage files first.")
		}

		// Read staged files
		stageData, err := os.ReadFile(stageFile)
		if err != nil {
			return fmt.Errorf("failed to read staged files: %w", err)
		}

		stagedFiles := strings.Fields(string(stageData))
		if len(stagedFiles) == 0 {
			return fmt.Errorf("no files staged for commit")
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
			return fmt.Errorf("failed to get current timeline: %w", err)
		}

		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		// Create commit using the new commit system
		fmt.Printf("Creating commit objects for %d staged files...\n", len(stagedFiles))
		
		// Initialize storage system with persistent file-based CAS
		objectsDir := filepath.Join(ivaldiDir, "objects")
		casStore, err := cas.NewFileCAS(objectsDir)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		mmr := history.NewMMR()
		commitBuilder := commit.NewCommitBuilder(casStore, mmr)
		
		// Create materializer to scan workspace
		materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
		
		// Scan the current workspace to create file metadata
		wsIndex, err := materializer.ScanWorkspace()
		if err != nil {
			return fmt.Errorf("failed to scan workspace: %w", err)
		}
		
		// Get workspace files
		wsLoader := wsindex.NewLoader(casStore)
		workspaceFiles, err := wsLoader.ListAll(wsIndex)
		if err != nil {
			return fmt.Errorf("failed to list workspace files: %w", err)
		}
		
		fmt.Printf("Found %d files in workspace\n", len(workspaceFiles))
		
		// Create commit object
		author := "Ivaldi User <user@example.com>" // TODO: get from config
		commitObj, err := commitBuilder.CreateCommit(
			workspaceFiles,
			nil, // TODO: get parent commits
			author,
			author,
			message,
		)
		if err != nil {
			return fmt.Errorf("failed to create commit: %w", err)
		}
		
		// Get commit hash
		commitHash := commitBuilder.GetCommitHash(commitObj)
		
		// Update timeline with the commit hash
		var commitHashArray [32]byte
		copy(commitHashArray[:], commitHash[:])
		
		// Update the timeline reference with commit hash
		err = refsManager.CreateTimeline(
			currentTimeline,
			refs.LocalTimeline,
			commitHashArray,
			[32]byte{}, // No SHA256 for now
			"",         // No Git SHA1
			fmt.Sprintf("Commit: %s", message),
		)
		if err != nil {
			// Timeline already exists, this is expected - in a real system we'd update it
			log.Printf("Note: Timeline update not yet implemented, but workspace state saved")
		}

		fmt.Printf("Successfully sealed commit on timeline '%s'\n", currentTimeline)
		fmt.Printf("Commit message: %s\n", message)
		fmt.Printf("Commit hash: %s\n", commitHash.String())

		// Status tracking is now handled by the workspace system

		// Clean up staging area
		if err := os.Remove(stageFile); err != nil {
			log.Printf("Warning: Failed to clean up staging area: %v", err)
		}

		return nil
	},
}

var excludeCommand = &cobra.Command{
	Use:   "exclude",
	Args:  cobra.MinimumNArgs(1),
	Short: "Excludes a file from gather",
	Long:  `Create a ivaldiignore file if it does exist and otherwise adds file to existing ignore file.`,
	RunE:  createOrAddExclude,
}

func createOrAddExclude(cmd *cobra.Command, args []string) error {
	ignoreFile := ".ivaldiignore"
	f, err := os.OpenFile(ignoreFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, pattern := range args {
		if _, err := f.WriteString(pattern + "\n"); err != nil {
			return fmt.Errorf("failed to write pattern '%s': %w", pattern, err)
		}
		fmt.Printf("Added '%s' to .ivaldiignore\n", pattern)
	}
	
	fmt.Printf("Successfully added %d patterns to .ivaldiignore\n", len(args))
	return nil
}

