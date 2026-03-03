package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/logging"
)

// CloneRepository clones a GitHub repository to the local workspace.
// Returns the actual default branch name from the repository.
func (rs *RepoSyncer) CloneRepository(ctx context.Context, owner, repo string, depth int, skipHistory bool, includeTags bool) (string, error) {
	fmt.Printf("Cloning %s/%s from GitHub...\n", owner, repo)

	// If skip-history is set, try to download archive directly without API calls
	// This avoids rate limits entirely for public repos
	if skipHistory {
		fmt.Println("Downloading latest snapshot (no API calls)...")

		// Try common default branch names directly with archive download.
		// This avoids API calls for the vast majority of repos.
		for _, branchName := range []string{"main", "master"} {
			fileCount, err := rs.downloadAndExtractArchive(ctx, owner, repo, branchName)
			if err == nil {
				fmt.Printf("Extracted %d files from archive (branch: %s)\n", fileCount, branchName)
				err = rs.createIvaldiCommit(fmt.Sprintf("Import from GitHub: %s/%s", owner, repo), branchName, "")
				if err != nil {
					return "", fmt.Errorf("failed to create Ivaldi commit: %w", err)
				}
				fmt.Printf("Successfully cloned snapshot from %s/%s\n", owner, repo)
				return branchName, nil
			}
		}

		// Neither "main" nor "master" worked.
		// Fall back to a single API call to discover the actual default branch.
		fmt.Println("Standard branch names not found, querying repository info...")
		rs.client.WaitForRateLimit()

		repoInfo, err := rs.client.GetRepository(ctx, owner, repo)
		if err != nil {
			return "", fmt.Errorf("failed to download repository (tried branches 'main' and 'master', then API lookup failed): %w\n\n"+
				"Note: This could mean:\n"+
				"  - The repository doesn't exist or is private\n"+
				"  - Check the repository name for typos\n"+
				"  - If private, run 'ivaldi auth login' first", err)
		}

		defaultBranch := repoInfo.DefaultBranch
		if defaultBranch == "" {
			return "", fmt.Errorf("repository '%s/%s' exists but has no default branch configured.\n"+
				"This usually means the repository is empty or misconfigured", owner, repo)
		}

		fmt.Printf("Repository uses non-standard default branch: '%s'\n", defaultBranch)

		fileCount, err := rs.downloadAndExtractArchive(ctx, owner, repo, defaultBranch)
		if err != nil {
			if repoInfo.Size == 0 {
				return "", fmt.Errorf("repository '%s/%s' exists but appears to be empty (no commits).\n"+
					"Initialize the repository on GitHub first, or push content to it before downloading", owner, repo)
			}
			return "", fmt.Errorf("failed to download repository archive for branch '%s': %w", defaultBranch, err)
		}

		fmt.Printf("Extracted %d files from archive (branch: %s)\n", fileCount, defaultBranch)
		err = rs.createIvaldiCommit(fmt.Sprintf("Import from GitHub: %s/%s", owner, repo), defaultBranch, "")
		if err != nil {
			return "", fmt.Errorf("failed to create Ivaldi commit: %w", err)
		}

		fmt.Printf("Successfully cloned snapshot from %s/%s\n", owner, repo)
		return defaultBranch, nil
	}

	// Check rate limits before API calls
	rs.client.WaitForRateLimit()

	// Get repository info
	repoInfo, err := rs.client.GetRepository(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository info: %w", err)
	}

	fmt.Printf("Repository: %s\n", repoInfo.FullName)
	if repoInfo.Description != "" {
		fmt.Printf("Description: %s\n", repoInfo.Description)
	}
	fmt.Printf("Default branch: %s\n", repoInfo.DefaultBranch)

	// Get the default branch
	branch, err := rs.client.GetBranch(ctx, owner, repo, repoInfo.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("failed to get branch info: %w", err)
	}

	// Check if we should skip history migration (backward compatibility)
	if skipHistory {
		fmt.Println("Skipping history migration, downloading latest snapshot only...")
		return repoInfo.DefaultBranch, rs.cloneSnapshot(ctx, owner, repo, branch.Commit.SHA, repoInfo.DefaultBranch)
	}

	// Fetch commit history
	fmt.Printf("\nFetching commit history (depth: ")
	if depth == 0 {
		fmt.Printf("full history")
	} else {
		fmt.Printf("%d commits", depth)
	}
	fmt.Println(")...")

	commits, err := rs.client.ListCommits(ctx, owner, repo, repoInfo.DefaultBranch, depth)
	if err != nil {
		return "", fmt.Errorf("failed to fetch commit history: %w", err)
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found in repository")
	}

	if depth == 0 {
		fmt.Printf("Retrieved complete history: %d commits\n\n", len(commits))
	} else {
		fmt.Printf("Found %d commits to import (limited by depth=%d)\n\n", len(commits), depth)
	}

	// Import commits in chronological order (reverse the list)
	err = rs.importCommitHistory(ctx, owner, repo, commits)
	if err != nil {
		return "", fmt.Errorf("failed to import commit history: %w", err)
	}

	// Import tags if requested
	if includeTags {
		fmt.Println("Importing tags and releases...")
		err = rs.importTags(ctx, owner, repo)
		if err != nil {
			logging.Warn("Failed to import tags", "error", err)
		}
	}

	fmt.Printf("Successfully cloned %s/%s with %d commits\n", owner, repo, len(commits))
	return repoInfo.DefaultBranch, nil
}

// cloneSnapshot downloads only the latest snapshot without history (backward compatibility)
func (rs *RepoSyncer) cloneSnapshot(ctx context.Context, owner, repo, commitSHA, branchName string) error {
	// Use archive download (no rate limits) instead of individual file downloads
	fileCount, err := rs.downloadAndExtractArchive(ctx, owner, repo, commitSHA)
	if err != nil {
		// Fallback to individual file downloads if archive fails
		fmt.Printf("Archive download failed (%v), falling back to API...\n", err)

		// Get the tree for the latest commit
		tree, err := rs.client.GetTree(ctx, owner, repo, commitSHA, true)
		if err != nil {
			return fmt.Errorf("failed to get repository tree: %w", err)
		}

		// Download files concurrently
		err = rs.downloadFiles(ctx, owner, repo, tree, commitSHA)
		if err != nil {
			return fmt.Errorf("failed to download files: %w", err)
		}
	} else {
		fmt.Printf("Extracted %d files from archive\n", fileCount)
	}

	// Create single initial commit in Ivaldi
	err = rs.createIvaldiCommit(fmt.Sprintf("Import from GitHub: %s/%s", owner, repo), branchName, commitSHA)
	if err != nil {
		return fmt.Errorf("failed to create Ivaldi commit: %w", err)
	}

	fmt.Printf("Successfully cloned snapshot from %s/%s\n", owner, repo)
	return nil
}

// extractTarGz extracts a gzipped tarball to the destination directory
func extractTarGz(archiveData []byte, destDir string) (int, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(archiveData))
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	fileCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fileCount, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// GitHub archives have a top-level directory like "repo-branch/"
		// Always strip the first path component
		name := header.Name

		// Find the first slash and strip everything before it (including the slash)
		slashIdx := strings.Index(name, "/")
		if slashIdx >= 0 {
			name = name[slashIdx+1:]
		} else {
			// Entry has no slash - it's the top-level dir name itself, skip it
			continue
		}

		// Skip empty names (this happens for the top-level directory entry)
		if name == "" {
			continue
		}

		targetPath := filepath.Join(destDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fileCount, fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			parentDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return fileCount, fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create the file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fileCount, fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fileCount, fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			outFile.Close()
			fileCount++

		case tar.TypeSymlink:
			// Handle symlinks
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fileCount, fmt.Errorf("failed to create parent directory for symlink: %w", err)
			}
			// Remove existing file/symlink if it exists
			os.Remove(targetPath)
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				// Symlink creation might fail on some systems, continue without error
				continue
			}
			fileCount++
		}
	}

	return fileCount, nil
}

// downloadAndExtractArchive downloads a repository archive and extracts it to the workspace
// This method does NOT use the GitHub API and therefore has no rate limits
func (rs *RepoSyncer) downloadAndExtractArchive(ctx context.Context, owner, repo, ref string) (int, error) {
	fmt.Printf("Downloading archive from codeload.github.com (no rate limit)...\n")

	archiveData, err := rs.client.DownloadArchive(ctx, owner, repo, ref)
	if err != nil {
		return 0, fmt.Errorf("failed to download archive: %w", err)
	}

	fmt.Printf("Downloaded %.2f MB, extracting...\n", float64(len(archiveData))/(1024*1024))

	fileCount, err := extractTarGz(archiveData, rs.workDir)
	if err != nil {
		return 0, fmt.Errorf("failed to extract archive: %w", err)
	}

	return fileCount, nil
}
