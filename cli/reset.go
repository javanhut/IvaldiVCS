package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset [<file>...]",
	Short: "Unstage files or reset working directory",
	Long: `Unstage files from the staging area.

Modes:
  ivaldi reset              # Unstage all files
  ivaldi reset <file>...    # Unstage specific files
  ivaldi reset --hard       # DANGER: Discard all uncommitted changes

Examples:
  ivaldi reset              # Unstage all files
  ivaldi reset file1.txt    # Unstage file1.txt
  ivaldi reset src/         # Unstage all files in src/`,
	RunE: runReset,
}

var resetHard bool

func init() {
	resetCmd.Flags().BoolVar(&resetHard, "hard", false, "DANGER: Discard all uncommitted changes")
}

func runReset(cmd *cobra.Command, args []string) error {
	// Check if we're in an Ivaldi repository
	ivaldiDir := ".ivaldi"
	if _, err := os.Stat(ivaldiDir); os.IsNotExist(err) {
		return fmt.Errorf("not in an Ivaldi repository (no .ivaldi directory found)")
	}

	// Handle --hard flag (dangerous operation)
	if resetHard {
		return resetHardMode(ivaldiDir)
	}

	// Handle unstaging
	if len(args) == 0 {
		// Reset all staged files
		return resetAll(ivaldiDir)
	}

	// Reset specific files
	return resetFiles(ivaldiDir, args)
}

// resetAll unstages all files
func resetAll(ivaldiDir string) error {
	stageFile := filepath.Join(ivaldiDir, "stage", "files")

	// Check if there are any staged files
	if _, err := os.Stat(stageFile); os.IsNotExist(err) {
		fmt.Println("No files staged.")
		return nil
	}

	// Read staged files to count them
	data, err := os.ReadFile(stageFile)
	if err != nil {
		return fmt.Errorf("failed to read staged files: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	// Remove the staging file
	if err := os.Remove(stageFile); err != nil {
		return fmt.Errorf("failed to remove staging file: %w", err)
	}

	fmt.Printf("%s %s\n",
		colors.SuccessText("Unstaged all files:"),
		colors.Bold(fmt.Sprintf("%d files", count)))
	fmt.Printf("%s\n", colors.Dim("Use 'ivaldi gather <file>...' to stage files again"))

	return nil
}

// resetFiles unstages specific files
func resetFiles(ivaldiDir string, filesToReset []string) error {
	stageFile := filepath.Join(ivaldiDir, "stage", "files")

	// Check if there are any staged files
	if _, err := os.Stat(stageFile); os.IsNotExist(err) {
		fmt.Println("No files staged.")
		return nil
	}

	// Read currently staged files
	data, err := os.ReadFile(stageFile)
	if err != nil {
		return fmt.Errorf("failed to read staged files: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var stagedFiles []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			stagedFiles = append(stagedFiles, line)
		}
	}

	// Build set of files to reset
	resetSet := make(map[string]bool)
	for _, file := range filesToReset {
		// Clean the path
		cleanPath := filepath.Clean(file)
		resetSet[cleanPath] = true

		// Also match directory prefixes
		for _, staged := range stagedFiles {
			if strings.HasPrefix(staged, cleanPath+"/") || staged == cleanPath {
				resetSet[staged] = true
			}
		}
	}

	// Filter out files to reset
	var remainingFiles []string
	var resetFilesList []string
	for _, staged := range stagedFiles {
		if resetSet[staged] {
			resetFilesList = append(resetFilesList, staged)
		} else {
			remainingFiles = append(remainingFiles, staged)
		}
	}

	if len(resetFilesList) == 0 {
		fmt.Println("No matching files to unstage.")
		return nil
	}

	// Write remaining files back to stage
	if len(remainingFiles) == 0 {
		// No files left, remove staging file
		if err := os.Remove(stageFile); err != nil {
			return fmt.Errorf("failed to remove staging file: %w", err)
		}
	} else {
		// Write remaining files
		content := strings.Join(remainingFiles, "\n") + "\n"
		if err := os.WriteFile(stageFile, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to update staging file: %w", err)
		}
	}

	// Show what was reset
	fmt.Printf("%s\n", colors.SuccessText("Unstaged files:"))
	for _, file := range resetFilesList {
		fmt.Printf("  %s\n", colors.InfoText(file))
	}
	fmt.Printf("\n%s %s\n",
		colors.Bold("Total:"),
		fmt.Sprintf("%d files unstaged", len(resetFilesList)))

	if len(remainingFiles) > 0 {
		fmt.Printf("%s\n", colors.Dim(fmt.Sprintf("%d files still staged", len(remainingFiles))))
	}

	return nil
}

// resetHardMode resets working directory to HEAD (dangerous!)
func resetHardMode(ivaldiDir string) error {
	// Confirmation prompt
	fmt.Println(colors.Red("WARNING: This will discard ALL uncommitted changes!"))
	fmt.Print("Are you sure? Type 'yes' to continue: ")

	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Reset cancelled.")
		return nil
	}

	// Clear staging area
	stageFile := filepath.Join(ivaldiDir, "stage", "files")
	if _, err := os.Stat(stageFile); err == nil {
		if err := os.Remove(stageFile); err != nil {
			return fmt.Errorf("failed to clear staging: %w", err)
		}
	}

	fmt.Println(colors.SuccessText("Cleared staging area."))
	fmt.Println()
	fmt.Println(colors.Yellow("Note: Full working directory reset not yet implemented."))
	fmt.Println(colors.Dim("Use 'ivaldi timeline switch <timeline>' to restore files from a timeline."))

	return nil
}
