package cli

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/commit"
	"github.com/javanhut/Ivaldi-vcs/internal/converter"
	"github.com/javanhut/Ivaldi-vcs/internal/github"
	"github.com/javanhut/Ivaldi-vcs/internal/history"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/javanhut/Ivaldi-vcs/internal/seals"
	"github.com/javanhut/Ivaldi-vcs/internal/workspace"
	"github.com/javanhut/Ivaldi-vcs/internal/wsindex"
	"github.com/spf13/cobra"
)

// isGitHubURL checks if the given URL is a GitHub repository URL
func isGitHubURL(rawURL string) bool {
	// Handle various GitHub URL formats
	patterns := []string{
		`^https?://github\.com/[\w-]+/[\w-]+`,
		`^git@github\.com:[\w-]+/[\w-]+`,
		`^github\.com/[\w-]+/[\w-]+`,
		`^[\w-]+/[\w-]+$`, // Simple owner/repo format
	}

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, rawURL)
		if matched {
			return true
		}
	}
	return false
}

// parseGitHubURL extracts owner and repo from various GitHub URL formats
func parseGitHubURL(rawURL string) (owner, repo string, err error) {
	// Remove .git suffix if present
	rawURL = strings.TrimSuffix(rawURL, ".git")

	// Handle simple owner/repo format
	if matched, _ := regexp.MatchString(`^[\w-]+/[\w-]+$`, rawURL); matched {
		parts := strings.Split(rawURL, "/")
		return parts[0], parts[1], nil
	}

	// Handle full URLs
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// Try adding https:// if not present
		if !strings.HasPrefix(rawURL, "http") && !strings.HasPrefix(rawURL, "git@") {
			parsedURL, err = url.Parse("https://" + rawURL)
			if err != nil {
				return "", "", fmt.Errorf("invalid URL: %s", rawURL)
			}
		} else if strings.HasPrefix(rawURL, "git@github.com:") {
			// Handle git@github.com:owner/repo format
			path := strings.TrimPrefix(rawURL, "git@github.com:")
			parts := strings.Split(path, "/")
			if len(parts) == 2 {
				return parts[0], parts[1], nil
			}
			return "", "", fmt.Errorf("invalid git URL format: %s", rawURL)
		} else {
			return "", "", err
		}
	}

	// Extract path and parse owner/repo
	path := strings.TrimPrefix(parsedURL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub URL format: %s", rawURL)
	}

	return parts[0], parts[1], nil
}

// handleGitHubDownload handles downloading/cloning from GitHub
func handleGitHubDownload(rawURL string, args []string) error {
	// Parse GitHub URL
	owner, repo, err := parseGitHubURL(rawURL)
	if err != nil {
		return fmt.Errorf("failed to parse GitHub URL: %w", err)
	}

	// Determine target directory
	targetDir := repo
	if len(args) > 1 {
		targetDir = args[1]
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Change to target directory
	if err := os.Chdir(targetDir); err != nil {
		return fmt.Errorf("failed to change directory: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Initialize Ivaldi repository
	ivaldiDir := ".ivaldi"
	if err := os.Mkdir(ivaldiDir, os.ModePerm); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to create .ivaldi directory: %w", err)
	}

	log.Println("Ivaldi repository initialized")

	// Initialize refs system
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		return fmt.Errorf("failed to initialize refs: %w", err)
	}
	defer refsManager.Close()

	// Create main timeline
	var zeroHash [32]byte
	err = refsManager.CreateTimeline(
		"main",
		refs.LocalTimeline,
		zeroHash,
		zeroHash,
		"",
		fmt.Sprintf("Clone from GitHub: %s/%s", owner, repo),
	)
	if err != nil {
		log.Printf("Warning: Failed to create main timeline: %v", err)
	}

	// Set main as current timeline
	if err := refsManager.SetCurrentTimeline("main"); err != nil {
		log.Printf("Warning: Failed to set current timeline: %v", err)
	}

	// Store GitHub repository configuration
	if err := refsManager.SetGitHubRepository(owner, repo); err != nil {
		log.Printf("Warning: Failed to store GitHub repository configuration: %v", err)
	} else {
		fmt.Printf("Configured repository for GitHub: %s/%s\n", owner, repo)
	}

	// Create syncer and clone
	syncer, err := github.NewRepoSyncer(ivaldiDir, workDir)
	if err != nil {
		return fmt.Errorf("failed to create syncer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fmt.Printf("Downloading from GitHub: %s/%s...\n", owner, repo)
	if err := syncer.CloneRepository(ctx, owner, repo); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Automatically detect and convert Git submodules (enabled by default)
	if recurseSubmodules {
		gitmodulesPath := filepath.Join(workDir, ".gitmodules")
		if _, err := os.Stat(gitmodulesPath); err == nil {
			log.Println("📦 Detected Git submodules, converting to Ivaldi format...")

			gitDir := filepath.Join(workDir, ".git")
			submoduleResult, err := converter.ConvertGitSubmodulesToIvaldi(
				gitDir,
				ivaldiDir,
				workDir,
				true, // recursive
			)

			if err != nil {
				log.Printf("Warning: Submodule conversion encountered errors: %v", err)
			}

			if submoduleResult != nil {
				if submoduleResult.Converted > 0 {
					log.Printf("✓ Converted %d Git submodules", submoduleResult.Converted)
				}
				if submoduleResult.ClonedModules > 0 {
					log.Printf("✓ Cloned %d missing submodules", submoduleResult.ClonedModules)
				}
				if submoduleResult.Skipped > 0 {
					log.Printf("⚠ Skipped %d submodules due to errors", submoduleResult.Skipped)
					for i, err := range submoduleResult.Errors {
						if i < 3 {
							log.Printf("  - %v", err)
						}
					}
					if len(submoduleResult.Errors) > 3 {
						log.Printf("  ... and %d more errors", len(submoduleResult.Errors)-3)
					}
				}
			}
		}
	}

	fmt.Printf("Successfully downloaded repository from GitHub\n")
	return nil
}

var uploadCmd = &cobra.Command{
	Use:     "upload [branch]",
	Aliases: []string{"push"},
	Short:   "Upload current timeline to GitHub",
	Long: `Uploads the current timeline to the configured GitHub repository. The repository is automatically detected from the configuration set during 'ivaldi download'.
Examples:
  ivaldi upload                           # Upload current timeline to GitHub
  ivaldi upload main                      # Upload to specific branch on GitHub
  ivaldi upload github:owner/repo         # Upload to different GitHub repository (current timeline)
  ivaldi upload github:owner/repo main    # Upload to different GitHub repository and branch`,
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

		// Auto-detect GitHub repository and branch
		var owner, repo, branch string
		branch = currentTimeline // Default branch to current timeline name

		// Check if GitHub repository is specified in arguments
		if len(args) > 0 && strings.HasPrefix(args[0], "github:") {
			// Parse GitHub repository from argument
			repoPath := strings.TrimPrefix(args[0], "github:")
			parts := strings.Split(repoPath, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid GitHub repository format. Use: github:owner/repo")
			}
			owner, repo = parts[0], parts[1]

			// Check if branch is specified
			if len(args) > 1 {
				branch = args[1]
			}
		} else {
			// Try to auto-detect GitHub repository from configuration
			var err error
			owner, repo, err = refsManager.GetGitHubRepository()
			if err != nil {
				return fmt.Errorf("no GitHub repository configured and none specified. Use 'ivaldi download' from GitHub first or specify 'github:owner/repo'")
			}

			// If first argument is not a GitHub URL, treat it as branch name
			if len(args) > 0 {
				branch = args[0]
			}
		}

		// Get current timeline's latest commit
		timeline, err := refsManager.GetTimeline(currentTimeline, refs.LocalTimeline)
		if err != nil {
			return fmt.Errorf("failed to get timeline info: %w", err)
		}

		if timeline.Blake3Hash == [32]byte{} {
			return fmt.Errorf("no commits to push")
		}

		// Convert to cas.Hash
		var commitHash cas.Hash
		copy(commitHash[:], timeline.Blake3Hash[:])

		// Create syncer and push
		syncer, err := github.NewRepoSyncer(ivaldiDir, workDir)
		if err != nil {
			return fmt.Errorf("failed to create syncer: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		fmt.Printf("Uploading to GitHub: %s/%s (branch: %s)...\n", owner, repo, branch)
		if err := syncer.PushCommit(ctx, owner, repo, branch, commitHash); err != nil {
			return fmt.Errorf("failed to push to GitHub: %w", err)
		}

		fmt.Printf("Successfully uploaded to GitHub\n")
		return nil
	},
}

var recurseSubmodules bool
var statusVerbose bool

var downloadCmd = &cobra.Command{
	Use:     "download <url> [directory]",
	Aliases: []string{"clone"},
	Short:   "Download/clone repository from remote",
	Long:    `Downloads a complete repository from a remote URL into a new directory. Supports GitHub repositories and standard Ivaldi remotes.`,
	Args:    cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		// Check if this is a GitHub URL
		if isGitHubURL(url) {
			return handleGitHubDownload(url, args)
		}

		// Standard Ivaldi remote download
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

		// TODO: Implement actual download/clone functionality for standard Ivaldi remotes
		fmt.Printf("Downloading repository from '%s' into '%s'...\n", url, targetDir)
		fmt.Println("Note: Standard Ivaldi remote download functionality not yet implemented.")

		return nil
	},
}

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

				// Skip directories
				if info.IsDir() {
					return nil
				}

				// Skip .ivaldi directory
				if strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) || relPath == ".ivaldi" {
					return nil
				}

				// Check if file is auto-excluded (.env, .venv, etc.)
				if isAutoExcluded(relPath) {
					log.Printf("Auto-excluded for security: %s", relPath)
					return nil
				}

				// Skip hidden files/dirs EXCEPT .ivaldiignore
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

						// Skip directories
						if info.IsDir() {
							return nil
						}

						// Skip hidden files and directories
						if strings.Contains(path, "/.") {
							return nil
						}

						// Get relative path from working directory
						relPath, err := filepath.Rel(workDir, path)
						if err != nil {
							return err
						}

						// Skip .ivaldi directory
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

		// Create materializer to scan workspace
		materializer := workspace.NewMaterializer(casStore, ivaldiDir, workDir)

		// Scan the current workspace to create file metadata
		wsIndex, err := materializer.ScanWorkspace()
		if err != nil {
			return fmt.Errorf("failed to scan workspace: %w", err)
		}

		// Get workspace files
		wsLoader := wsindex.NewLoader(casStore)
		allWorkspaceFiles, err := wsLoader.ListAll(wsIndex)
		if err != nil {
			return fmt.Errorf("failed to list workspace files: %w", err)
		}

		// Filter workspace files to only include staged files
		stagedFileMap := make(map[string]bool)
		for _, file := range stagedFiles {
			stagedFileMap[file] = true
		}

		var workspaceFiles []wsindex.FileMetadata
		for _, file := range allWorkspaceFiles {
			if stagedFileMap[file.Path] {
				workspaceFiles = append(workspaceFiles, file)
			}
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

func init() {
	statusCmd.Flags().BoolVar(&statusVerbose, "verbose", false, "Show more detailed status information")
	downloadCmd.Flags().BoolVar(&recurseSubmodules, "recurse-submodules", true, "Automatically clone and convert Git submodules (default: true)")
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
