package cli

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/spf13/cobra"
)

// Auto-excluded patterns that are always ignored for security
var autoExcludePatterns = []string{
	".env",
	".env.*",
	".venv",
	".venv/",
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

		// Get --allow-all flag
		allowAll, err := cmd.Flags().GetBool("allow-all")
		if err != nil {
			return fmt.Errorf("failed to get allow-all flag: %w", err)
		}

		// Load ignore patterns from .ivaldiignore
		ignorePatterns, err := loadIgnorePatternsForGather(workDir)
		if err != nil {
			log.Printf("Warning: Failed to load ignore patterns: %v", err)
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

				// Get relative path
				relPath, err := filepath.Rel(workDir, path)
				if err != nil {
					return err
				}

				// Handle directories - check exclusions BEFORE deciding to skip
				if info.IsDir() {
					// Skip .ivaldi directory
					if relPath == ".ivaldi" || strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) {
						return filepath.SkipDir
					}

					// Check if directory is auto-excluded
					if isAutoExcluded(relPath) {
						log.Printf("Auto-excluded directory for security: %s", relPath)
						return filepath.SkipDir
					}

					// Check if directory matches ignore patterns
					// Try both with and without trailing slash
					if isFileIgnored(relPath, ignorePatterns) || isFileIgnored(relPath+"/", ignorePatterns) {
						log.Printf("Skipping ignored directory: %s", relPath)
						return filepath.SkipDir
					}

					// Check for hidden directories (except .ivaldiignore parent)
					if filepath.Base(path)[0] == '.' && relPath != "." {
						if !allowAll {
							log.Printf("Skipping hidden directory: %s", relPath)
							return filepath.SkipDir
						}
					}

					// Directory is not excluded, continue into it
					return nil
				}

				// From here on, we're dealing with files only

				// Skip .ivaldi directory files (shouldn't happen but just in case)
				if strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) || relPath == ".ivaldi" {
					return nil
				}

				// Check if file is auto-excluded (.env, .venv, etc.)
				if isAutoExcluded(relPath) {
					log.Printf("Auto-excluded for security: %s", relPath)
					return nil
				}

				// Skip hidden files EXCEPT .ivaldiignore
				if filepath.Base(path)[0] == '.' && relPath != ".ivaldiignore" {
					// Prompt user for dot files unless --allow-all is set
					if !allowAll {
						if shouldGatherDotFile(relPath) {
							filesToGather = append(filesToGather, relPath)
						}
						return nil
					} else {
						// With --allow-all, still warn about dot files
						fmt.Printf("Warning: Gathering hidden file: %s\n", relPath)
					}
				}

				// Skip ignored files (but never ignore .ivaldiignore itself)
				if isFileIgnored(relPath, ignorePatterns) {
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
			for _, arg := range args {
				// Convert relative paths to absolute for consistency
				absPath := arg
				if !filepath.IsAbs(arg) {
					absPath = filepath.Join(workDir, arg)
				}

				info, err := os.Stat(absPath)
				if os.IsNotExist(err) {
					log.Printf("Warning: File '%s' does not exist, skipping", arg)
					continue
				}

				if info.IsDir() {
					// If it's a directory, walk it and add all files
					err := filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}

						// Get relative path from working directory
						relPath, err := filepath.Rel(workDir, path)
						if err != nil {
							return err
						}

						// Handle directories - check exclusions BEFORE deciding to skip
						if info.IsDir() {
							// Skip .ivaldi directory
							if relPath == ".ivaldi" || strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) {
								return filepath.SkipDir
							}

							// Check if directory is auto-excluded
							if isAutoExcluded(relPath) {
								log.Printf("Auto-excluded directory for security: %s", relPath)
								return filepath.SkipDir
							}

							// Check if directory matches ignore patterns
							if isFileIgnored(relPath, ignorePatterns) || isFileIgnored(relPath+"/", ignorePatterns) {
								log.Printf("Skipping ignored directory: %s", relPath)
								return filepath.SkipDir
							}

							// Check for hidden directories
							if strings.Contains(path, "/.") && relPath != "." {
								if !allowAll {
									log.Printf("Skipping hidden directory: %s", relPath)
									return filepath.SkipDir
								}
							}

							// Directory is not excluded, continue into it
							return nil
						}

						// From here on, we're dealing with files only

						// Skip hidden files and directories
						if strings.Contains(path, "/.") {
							return nil
						}

						// Skip .ivaldi directory files
						if strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) || relPath == ".ivaldi" {
							return nil
						}

						// Check if file is auto-excluded
						if isAutoExcluded(relPath) {
							log.Printf("Auto-excluded for security: %s", relPath)
							return nil
						}

						// Check for dot files (except .ivaldiignore)
						if strings.Contains(path, "/.") && relPath != ".ivaldiignore" {
							if !allowAll {
								if shouldGatherDotFile(relPath) {
									filesToGather = append(filesToGather, relPath)
								}
								return nil
							} else {
								fmt.Printf("Warning: Gathering hidden file: %s\n", relPath)
							}
						}

						// Skip ignored files (but never ignore .ivaldiignore itself)
						if isFileIgnored(relPath, ignorePatterns) {
							log.Printf("Skipping ignored file: %s", relPath)
							return nil
						}

						filesToGather = append(filesToGather, relPath)
						return nil
					})
					if err != nil {
						log.Printf("Warning: Failed to walk directory '%s': %v", arg, err)
					}
				} else {
					// It's a file, get relative path
					relPath, err := filepath.Rel(workDir, arg)
					if err != nil {
						// If we can't get relative path, use as-is
						relPath = arg
					}

					// Check if file is auto-excluded
					if isAutoExcluded(relPath) {
						log.Printf("Warning: File '%s' is auto-excluded for security, skipping", relPath)
						continue
					}

					// Check for dot files (except .ivaldiignore)
					if (filepath.Base(relPath)[0] == '.' || strings.Contains(relPath, "/.")) && relPath != ".ivaldiignore" {
						if !allowAll {
							if !shouldGatherDotFile(relPath) {
								continue
							}
						} else {
							fmt.Printf("Warning: Gathering hidden file: %s\n", relPath)
						}
					}

					// Check if file is ignored
					if isFileIgnored(relPath, ignorePatterns) {
						log.Printf("Warning: File '%s' is in .ivaldiignore, skipping", relPath)
						continue
					}

					filesToGather = append(filesToGather, relPath)
				}
			}
		}

		if len(filesToGather) == 0 {
			fmt.Println("No files to gather.")
			return nil
		}

		// Read existing staged files
		stageFile := filepath.Join(stageDir, "files")
		existingStaged := make(map[string]bool)
		if data, err := os.ReadFile(stageFile); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					existingStaged[line] = true
				}
			}
		}

		// Add new files to staging
		for _, file := range filesToGather {
			existingStaged[file] = true
		}

		// Write all staged files
		f, err := os.Create(stageFile)
		if err != nil {
			return fmt.Errorf("failed to create stage file: %w", err)
		}
		defer f.Close()

		stagedCount := 0
		for file := range existingStaged {
			if _, err := f.WriteString(file + "\n"); err != nil {
				return fmt.Errorf("failed to write to stage file: %w", err)
			}
			// Only print for newly gathered files
			found := false
			for _, newFile := range filesToGather {
				if newFile == file {
					fmt.Printf("Gathered: %s\n", file)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("Already staged: %s\n", file)
			}
			stagedCount++
		}

		fmt.Printf("Successfully gathered %d files for staging (total staged: %d).\n", len(filesToGather), stagedCount)
		fmt.Println("Use 'ivaldi seal <message>' to create a commit with these files.")

		return nil
	},
}

func init() {
	gatherCmd.Flags().Bool("allow-all", false, "Allow gathering all hidden files without prompting")
}

// isAutoExcluded checks if a file matches auto-exclude patterns (.env, .venv, etc.)
func isAutoExcluded(path string) bool {
	baseName := filepath.Base(path)

	for _, pattern := range autoExcludePatterns {
		// Handle directory patterns
		if strings.HasSuffix(pattern, "/") {
			dirPattern := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(path, dirPattern+"/") || baseName == dirPattern {
				return true
			}
		}

		// Try matching the basename
		if matched, _ := filepath.Match(pattern, baseName); matched {
			return true
		}

		// Try matching the full path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	return false
}

// shouldGatherDotFile prompts the user whether to gather a dot file
// Returns true if user wants to gather the file
func shouldGatherDotFile(path string) bool {
	fmt.Printf("\n%s '%s' is a hidden file.\n", colors.Yellow("Warning:"), colors.Bold(path))
	fmt.Print("Do you want to gather this file? (y/N): ")

	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "y" || response == "yes" {
		fmt.Printf("%s Gathering: %s\n", colors.Green("✓"), path)
		return true
	}

	fmt.Printf("%s Skipped: %s\n", colors.Gray("✗"), path)
	return false
}

// loadIgnorePatternsForGather loads patterns from .ivaldiignore file
func loadIgnorePatternsForGather(workDir string) ([]string, error) {
	ignoreFile := filepath.Join(workDir, ".ivaldiignore")
	if _, err := os.Stat(ignoreFile); os.IsNotExist(err) {
		return []string{}, nil // No ignore file
	}

	file, err := os.Open(ignoreFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns, scanner.Err()
}

// isFileIgnored checks if a file path matches any ignore patterns
// IMPORTANT: .ivaldiignore itself is NEVER ignored
func isFileIgnored(path string, patterns []string) bool {
	// Never ignore .ivaldiignore itself
	if path == ".ivaldiignore" || filepath.Base(path) == ".ivaldiignore" {
		return false
	}

	for _, pattern := range patterns {
		// Handle directory patterns (patterns ending with /)
		if strings.HasSuffix(pattern, "/") {
			dirPattern := strings.TrimSuffix(pattern, "/")
			// Check if the path is within this directory
			if strings.HasPrefix(path, dirPattern+"/") || path == dirPattern {
				return true
			}
		}

		// Try matching the full path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}

		// Try matching just the basename
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}

		// Handle patterns with directory separators
		if strings.Contains(pattern, "/") {
			if matched, _ := filepath.Match(pattern, path); matched {
				return true
			}
		}

		// Handle wildcards in directory paths (e.g., **/*.log)
		if strings.Contains(pattern, "**") {
			// Convert ** pattern to a simpler check
			parts := strings.Split(pattern, "**")
			if len(parts) == 2 {
				prefix := strings.TrimPrefix(parts[0], "/")
				suffix := strings.TrimPrefix(parts[1], "/")

				if prefix != "" && !strings.HasPrefix(path, prefix) {
					continue
				}

				if suffix != "" {
					if matched, _ := filepath.Match(suffix, filepath.Base(path)); matched {
						return true
					}
				}
			}
		}
	}
	return false
}
