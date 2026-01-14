package cli

import (
	"bufio"
	"context"
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
	"github.com/javanhut/Ivaldi-vcs/internal/converter"
	"github.com/javanhut/Ivaldi-vcs/internal/gitclone"
	"github.com/javanhut/Ivaldi-vcs/internal/github"
	"github.com/javanhut/Ivaldi-vcs/internal/gitlab"
	"github.com/javanhut/Ivaldi-vcs/internal/refs"
	"github.com/spf13/cobra"
)

var recurseSubmodules bool
var forceUpload bool

// isGitURL checks if the URL is a generic Git repository URL
func isGitURL(rawURL string) bool {
	// Detect generic Git URLs
	patterns := []string{
		`^https?://.*\.git$`, // https://server.com/repo.git
		`^git://`,            // git://server.com/repo
		`^ssh://git@`,        // ssh://git@server.com/repo
		`^git@[\w\.-]+:`,     // git@server.com:user/repo.git
	}

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, rawURL)
		if matched {
			return true
		}
	}

	// Also check for https://any-server.com/path (not GitHub/GitLab)
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		// Not GitHub or GitLab - likely a generic Git server
		if !isGitHubURL(rawURL) && !isGitLabURL(rawURL) {
			return true
		}
	}

	return false
}

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
func handleGitHubDownload(rawURL string, args []string, depth int, skipHistory bool, includeTags bool) error {
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

	// Save original directory for cleanup on failure
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Track if we created the directory (for cleanup)
	createdDir := false

	// Cleanup function to remove directory on failure
	cleanup := func() {
		if createdDir {
			// Change back to original directory first
			os.Chdir(originalDir)
			// Remove the target directory
			if err := os.RemoveAll(filepath.Join(originalDir, targetDir)); err != nil {
				log.Printf("Warning: Failed to cleanup directory '%s': %v", targetDir, err)
			} else {
				log.Printf("Cleaned up incomplete download directory: %s", targetDir)
			}
		}
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	createdDir = true

	// Change to target directory
	if err := os.Chdir(targetDir); err != nil {
		cleanup()
		return fmt.Errorf("failed to change directory: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Initialize Ivaldi repository
	ivaldiDir := ".ivaldi"
	if err := os.Mkdir(ivaldiDir, os.ModePerm); err != nil && !os.IsNotExist(err) {
		cleanup()
		return fmt.Errorf("failed to create .ivaldi directory: %w", err)
	}

	log.Println("Ivaldi repository initialized")

	// Initialize refs system
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		cleanup()
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

	// Create syncer for cloning (uses optional auth - works for public repos without login)
	syncer, err := github.NewRepoSyncerForClone(ivaldiDir, workDir)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to create syncer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	fmt.Printf("Downloading from GitHub: %s/%s...\n", owner, repo)
	if err := syncer.CloneRepository(ctx, owner, repo, depth, skipHistory, includeTags); err != nil {
		cleanup()
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

// isGitLabURL checks if a URL is a GitLab URL
func isGitLabURL(rawURL string) bool {
	// Handle various GitLab URL formats
	patterns := []string{
		`^https?://gitlab\.com/[\w-]+/[\w-]+`,
		`^git@gitlab\.com:[\w-]+/[\w-]+`,
		`^gitlab\.com/[\w-]+/[\w-]+`,
	}

	for _, pattern := range patterns {
		matched, _ := regexp.MatchString(pattern, rawURL)
		if matched {
			return true
		}
	}
	return false
}

// handleGitLabDownload handles downloading/cloning from GitLab
func handleGitLabDownload(rawURL string, args []string, baseURL string, depth int, skipHistory bool, includeTags bool) error {
	// Parse GitLab URL with host detection
	owner, repo, detectedHost, err := gitlab.ParseGitLabURLWithHost(rawURL)
	if err != nil {
		return fmt.Errorf("failed to parse GitLab URL: %w", err)
	}

	// Use detected host if no explicit baseURL was provided via --url flag
	// and the detected host is not the default gitlab.com
	if baseURL == "" && detectedHost != "" && detectedHost != "gitlab.com" {
		baseURL = detectedHost
	}

	// Determine target directory
	targetDir := repo
	if len(args) > 1 {
		targetDir = args[1]
	}

	// Save original directory for cleanup on failure
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Track if we created the directory (for cleanup)
	createdDir := false

	// Cleanup function to remove directory on failure
	cleanup := func() {
		if createdDir {
			os.Chdir(originalDir)
			if err := os.RemoveAll(filepath.Join(originalDir, targetDir)); err != nil {
				log.Printf("Warning: Failed to cleanup directory '%s': %v", targetDir, err)
			} else {
				log.Printf("Cleaned up incomplete download directory: %s", targetDir)
			}
		}
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	createdDir = true

	// Change to target directory
	if err := os.Chdir(targetDir); err != nil {
		cleanup()
		return fmt.Errorf("failed to change directory: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Initialize Ivaldi repository
	ivaldiDir := ".ivaldi"
	if err := os.Mkdir(ivaldiDir, os.ModePerm); err != nil && !os.IsNotExist(err) {
		cleanup()
		return fmt.Errorf("failed to create .ivaldi directory: %w", err)
	}

	log.Println("Ivaldi repository initialized")

	// Initialize refs system
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		cleanup()
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
		fmt.Sprintf("Clone from GitLab: %s/%s", owner, repo),
	)
	if err != nil {
		log.Printf("Warning: Failed to create main timeline: %v", err)
	}

	// Set main as current timeline
	if err := refsManager.SetCurrentTimeline("main"); err != nil {
		log.Printf("Warning: Failed to set current timeline: %v", err)
	}

	// Store GitLab repository configuration with custom URL if provided
	if baseURL != "" {
		if err := refsManager.SetGitLabRepositoryWithURL(owner, repo, baseURL); err != nil {
			log.Printf("Warning: Failed to store GitLab repository configuration: %v", err)
		} else {
			fmt.Printf("Configured repository for GitLab: %s/%s (URL: %s)\n", owner, repo, baseURL)
		}
	} else {
		if err := refsManager.SetGitLabRepository(owner, repo); err != nil {
			log.Printf("Warning: Failed to store GitLab repository configuration: %v", err)
		} else {
			fmt.Printf("Configured repository for GitLab: %s/%s\n", owner, repo)
		}
	}

	// Create syncer and clone
	var syncer *gitlab.RepoSyncer
	if baseURL != "" {
		syncer, err = gitlab.NewRepoSyncerWithURL(ivaldiDir, workDir, owner, repo, baseURL)
	} else {
		syncer, err = gitlab.NewRepoSyncer(ivaldiDir, workDir, owner, repo)
	}
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to create syncer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if baseURL != "" {
		fmt.Printf("Downloading from GitLab (%s): %s/%s...\n", baseURL, owner, repo)
	} else {
		fmt.Printf("Downloading from GitLab: %s/%s...\n", owner, repo)
	}
	if err := syncer.CloneRepository(ctx, owner, repo, depth, skipHistory, includeTags); err != nil {
		cleanup()
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	fmt.Printf("Successfully downloaded repository from GitLab\n")
	return nil
}

// handleGenericGitDownload handles downloading from any Git server
func handleGenericGitDownload(rawURL string, args []string, depth int, skipHistory bool, includeTags bool, username, password, token, sshKey string) error {
	// Determine target directory
	targetDir := extractRepoName(rawURL)
	if len(args) > 1 {
		targetDir = args[1]
	}

	// Save original directory for cleanup on failure
	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Track if we created the directory (for cleanup)
	createdDir := false

	// Cleanup function to remove directory on failure
	cleanup := func() {
		if createdDir {
			os.Chdir(originalDir)
			if err := os.RemoveAll(filepath.Join(originalDir, targetDir)); err != nil {
				log.Printf("Warning: Failed to cleanup directory '%s': %v", targetDir, err)
			} else {
				log.Printf("Cleaned up incomplete download directory: %s", targetDir)
			}
		}
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	createdDir = true

	// Change to target directory
	if err := os.Chdir(targetDir); err != nil {
		cleanup()
		return fmt.Errorf("failed to change directory: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Initialize Ivaldi repository
	ivaldiDir := ".ivaldi"
	if err := os.Mkdir(ivaldiDir, os.ModePerm); err != nil && !os.IsExist(err) {
		cleanup()
		return fmt.Errorf("failed to create .ivaldi directory: %w", err)
	}

	log.Println("Ivaldi repository initialized")

	// Initialize refs system
	refsManager, err := refs.NewRefsManager(ivaldiDir)
	if err != nil {
		cleanup()
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
		fmt.Sprintf("Clone from Git: %s", rawURL),
	)
	if err != nil {
		log.Printf("Warning: Failed to create main timeline: %v", err)
	}

	// Set main as current timeline
	if err := refsManager.SetCurrentTimeline("main"); err != nil {
		log.Printf("Warning: Failed to set current timeline: %v", err)
	}

	// Create cloner
	cloner, err := gitclone.NewCloner(ivaldiDir, workDir)
	if err != nil {
		cleanup()
		return fmt.Errorf("failed to create cloner: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	fmt.Printf("Downloading from Git repository: %s...\n", rawURL)

	// Clone options
	cloneOpts := &gitclone.CloneOptions{
		URL:         rawURL,
		Depth:       depth,
		SkipHistory: skipHistory,
		IncludeTags: includeTags,
		Username:    username,
		Password:    password,
		Token:       token,
		SSHKey:      sshKey,
	}

	if err := cloner.Clone(ctx, cloneOpts); err != nil {
		cleanup()
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	fmt.Printf("Successfully downloaded repository from Git server\n")
	return nil
}

// extractRepoName extracts repository name from Git URL
func extractRepoName(url string) string {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Handle different URL formats
	if strings.HasPrefix(url, "git@") {
		// git@server.com:user/repo -> repo
		parts := strings.Split(url, ":")
		if len(parts) > 1 {
			path := parts[len(parts)-1]
			pathParts := strings.Split(path, "/")
			return pathParts[len(pathParts)-1]
		}
	}

	// Handle HTTP(S) and other URLs
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

var uploadCmd = &cobra.Command{
	Use:     "upload [branch]",
	Aliases: []string{"push"},
	Short:   "Upload current timeline to GitHub",
	Long: `Uploads the current timeline to the configured GitHub repository. The repository is automatically detected from the configuration set during 'ivaldi download'.
Examples:
  ivaldi upload                           # Upload current timeline to GitHub
  ivaldi upload main                      # Upload to specific branch on GitHub`,
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

		// Auto-detect GitHub repository from portal configuration
		var owner, repo, branch string
		branch = currentTimeline // Default branch to current timeline name

		// Get GitHub repository from configuration
		owner, repo, err = refsManager.GetGitHubRepository()
		if err != nil {
			return fmt.Errorf("no GitHub repository configured. Use 'ivaldi portal add owner/repo' or download from GitHub first")
		}

		// If argument provided, treat it as branch name
		if len(args) > 0 {
			branch = args[0]
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

		// Force push safety checks
		if forceUpload {
			fmt.Printf("\n%s Force push will OVERWRITE remote history!\n",
				colors.Yellow("⚠ WARNING:"))
			fmt.Println("This is a destructive operation that:")
			fmt.Println("  • Rewrites commit history on the remote")
			fmt.Println("  • Can cause issues for collaborators")
			fmt.Println("  • Cannot be undone easily")
			fmt.Println()
			fmt.Printf("%s Consider creating a backup branch first:\n",
				colors.Bold("💡 Tip:"))
			fmt.Printf("  ivaldi timeline create backup-before-force-push\n")
			fmt.Printf("  ivaldi upload backup-before-force-push\n\n")

			// Require explicit confirmation
			fmt.Print("Type 'force push' to confirm: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			confirmation := strings.TrimSpace(input)
			if confirmation != "force push" {
				fmt.Println("Force push cancelled.")
				return nil
			}
		}

		fmt.Printf("Uploading to GitHub: %s/%s (branch: %s)...\n", owner, repo, branch)
		if err := syncer.PushCommit(ctx, owner, repo, branch, commitHash, forceUpload); err != nil {
			return fmt.Errorf("failed to push to GitHub: %w", err)
		}

		if forceUpload {
			fmt.Printf("\n%s Force pushed to GitHub\n", colors.Green("✓"))
			fmt.Printf("%s Make sure to notify collaborators about the history rewrite\n",
				colors.Yellow("⚠"))
		} else {
			fmt.Printf("Successfully uploaded to GitHub\n")
		}
		return nil
	},
}

var downloadCmd = &cobra.Command{
	Use:     "download <url> [directory]",
	Aliases: []string{"clone"},
	Short:   "Download/clone repository from remote",
	Long: `Downloads a repository from a remote URL into a new directory.
Supports GitHub, GitLab, and generic Git repositories.

By default, downloads only the latest snapshot (no commit history).
Use --with-history to download full commit history (requires API, subject to rate limits).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		gitlabFlag, _ := cmd.Flags().GetBool("gitlab")
		customURL, _ := cmd.Flags().GetString("url")
		depth, _ := cmd.Flags().GetInt("depth")
		withHistory, _ := cmd.Flags().GetBool("with-history")
		includeTags, _ := cmd.Flags().GetBool("include-tags")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		token, _ := cmd.Flags().GetString("token")
		sshKey, _ := cmd.Flags().GetString("ssh-key")

		// Invert: skipHistory is true when withHistory is false (default behavior is snapshot only)
		skipHistory := !withHistory

		// Check --gitlab flag for explicit GitLab handling
		if gitlabFlag {
			return handleGitLabDownload(url, args, customURL, depth, skipHistory, includeTags)
		}

		// Auto-detect platform from URL
		if isGitHubURL(url) {
			return handleGitHubDownload(url, args, depth, skipHistory, includeTags)
		}

		if isGitLabURL(url) {
			return handleGitLabDownload(url, args, customURL, depth, skipHistory, includeTags)
		}

		// Check if it's a generic Git URL
		if isGitURL(url) {
			return handleGenericGitDownload(url, args, depth, skipHistory, includeTags, username, password, token, sshKey)
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

func init() {
	downloadCmd.Flags().BoolVar(&recurseSubmodules, "recurse-submodules", true, "Automatically clone and convert Git submodules (default: true)")
	downloadCmd.Flags().Bool("gitlab", false, "Download from GitLab instead of GitHub")
	downloadCmd.Flags().String("url", "", "Custom GitLab instance URL (e.g., gitlab.javanstormbreaker.com)")
	downloadCmd.Flags().Int("depth", 0, "Limit commit history depth when using --with-history (0 for full history)")
	downloadCmd.Flags().Bool("with-history", false, "Download full commit history (requires API calls, subject to rate limits)")
	downloadCmd.Flags().Bool("include-tags", false, "Include tags and releases in the import (requires --with-history)")

	// Generic Git authentication flags
	downloadCmd.Flags().String("username", "", "Username for HTTP basic authentication")
	downloadCmd.Flags().String("password", "", "Password for HTTP basic authentication")
	downloadCmd.Flags().String("token", "", "Personal access token for authentication")
	downloadCmd.Flags().String("ssh-key", "", "Path to SSH private key (default: ~/.ssh/id_rsa)")

	uploadCmd.Flags().BoolVar(&forceUpload, "force", false, "Force push to remote (overwrites remote history - use with caution!)")
}
