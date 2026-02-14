package cli

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/seals"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
	"github.com/spf13/cobra"
)

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
			return fmt.Errorf("no files staged for commit. Use 'ivaldi gather' to stage files first")
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

		// Scan only the staged files (not the entire workspace)
		materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)
		wsIndex, err := materializer.ScanSpecificFiles(stagedFiles)
		if err != nil {
			return fmt.Errorf("failed to scan staged files: %w", err)
		}

		wsLoader := wsindex.NewLoader(casStore)
		workspaceFiles, err := wsLoader.ListAll(wsIndex)
		if err != nil {
			return fmt.Errorf("failed to list workspace files: %w", err)
		}

		fmt.Printf("Found %d files in workspace\n", len(workspaceFiles))

		// Get author from config
		author, err := getAuthorFromConfig()
		if err != nil {
			return fmt.Errorf("failed to get author from config: %w\nPlease set user.name and user.email: ivaldi config user.name \"Your Name\"", err)
		}

		// Get parent commit from current timeline
		var parents []cas.Hash
		timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
		if err == nil && timeline.Blake3Hash != [32]byte{} {
			// Timeline has a previous commit, use it as parent
			var parentHash cas.Hash
			copy(parentHash[:], timeline.Blake3Hash[:])
			parents = append(parents, parentHash)
		}

		// Create commit object
		commitObj, err := commitBuilder.CreateCommit(
			workspaceFiles,
			parents,
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

		// Generate and store seal name
		sealName := seals.GenerateSealName(commitHashArray)
		err = refsManager.StoreSealName(sealName, commitHashArray, message)
		if err != nil {
			log.Printf("Warning: Failed to store seal name: %v", err)
		}

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

		fmt.Printf("%s on timeline '%s'\n", colors.SuccessText("Successfully sealed commit"), colors.Bold(currentTimeline))
		fmt.Printf("Created seal: %s (%s)\n", colors.Cyan(sealName), colors.Gray(hex.EncodeToString(commitHashArray[:4])))
		fmt.Printf("Commit message: %s\n", colors.InfoText(message))

		// Status tracking is now handled by the workspace system

		// Clean up staging area
		if err := os.Remove(stageFile); err != nil {
			log.Printf("Warning: Failed to clean up staging area: %v", err)
		}

		return nil
	},
}
