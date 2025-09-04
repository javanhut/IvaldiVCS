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
	rootCmd.AddCommand(initialCmd)
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
			
			// Convert Git objects (skip for now to avoid database locking issues)
			log.Printf("Skipping Git object conversion due to database locking - will be implemented in separate command")
		} else {
			// Initialize default timeline for new repository
			log.Println("Creating default 'main' timeline...")
			// Create an empty main branch (we'd need a proper commit for this in real implementation)
			if err := refsManager.SetCurrentTimeline("main"); err != nil {
				log.Printf("Warning: Failed to set current timeline: %v", err)
			}
		}
	}

	// Create snapshot of current files
	log.Println("Creating snapshot of current files...")
	result, err := converter.SnapshotCurrentFiles(workDir, ivaldiDir)
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
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
