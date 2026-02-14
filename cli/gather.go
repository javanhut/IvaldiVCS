package cli

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/javanhut/Ivaldi-vcs/internal/colors"
	"github.com/javanhut/Ivaldi-vcs/internal/ignore"
	"github.com/spf13/cobra"
)

// fileResult holds a file path discovered during parallel walking
type fileResult struct {
	path string
	err  error
}

// dirJob represents a directory to be processed
type dirJob struct {
	path string
}

// parallelWalker performs parallel directory traversal
type parallelWalker struct {
	workDir       string
	allowAll      bool
	patternCache  *ignore.PatternCache
	results       chan fileResult
	jobs          chan dirJob
	wg            sync.WaitGroup
	workerCount   int
	dotFileAsks   chan string
	dotFileResult chan bool
}

// newParallelWalker creates a new parallel walker
func newParallelWalker(workDir string, allowAll bool, patternCache *ignore.PatternCache) *parallelWalker {
	workerCount := runtime.NumCPU()
	if workerCount < 4 {
		workerCount = 4
	}

	return &parallelWalker{
		workDir:       workDir,
		allowAll:      allowAll,
		patternCache:  patternCache,
		results:       make(chan fileResult, 1000),
		jobs:          make(chan dirJob, 1000),
		workerCount:   workerCount,
		dotFileAsks:   make(chan string),
		dotFileResult: make(chan bool),
	}
}

// walk performs the parallel directory walk
func (pw *parallelWalker) walk() []string {
	// Start workers
	for i := 0; i < pw.workerCount; i++ {
		pw.wg.Add(1)
		go pw.worker()
	}

	// Start with root directory
	pw.jobs <- dirJob{path: pw.workDir}

	// Start collector goroutine
	var files []string
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		for result := range pw.results {
			if result.err != nil {
				log.Printf("Warning: %v", result.err)
				continue
			}
			mu.Lock()
			files = append(files, result.path)
			mu.Unlock()
		}
		close(done)
	}()

	// Handle dot file prompts in main goroutine (for user interaction)
	go func() {
		for path := range pw.dotFileAsks {
			pw.dotFileResult <- shouldGatherDotFile(path)
		}
	}()

	// Wait for all workers to finish
	pw.wg.Wait()
	close(pw.jobs)
	close(pw.results)
	close(pw.dotFileAsks)

	<-done

	return files
}

// worker processes directory jobs
func (pw *parallelWalker) worker() {
	defer pw.wg.Done()

	for job := range pw.jobs {
		pw.processDir(job.path)
	}
}

// processDir processes a single directory
func (pw *parallelWalker) processDir(dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		pw.results <- fileResult{err: fmt.Errorf("failed to read directory %s: %w", dirPath, err)}
		return
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())
		relPath, err := filepath.Rel(pw.workDir, fullPath)
		if err != nil {
			continue
		}

		if entry.IsDir() {
			// Skip .ivaldi directory
			if relPath == ".ivaldi" || strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) {
				continue
			}

			// Check if directory is auto-excluded
			if isAutoExcluded(relPath) {
				log.Printf("Auto-excluded directory for security: %s", relPath)
				continue
			}

			// Check if directory matches ignore patterns
			if pw.patternCache.IsIgnored(relPath) || pw.patternCache.IsIgnored(relPath+"/") {
				log.Printf("Skipping ignored directory: %s", relPath)
				continue
			}

			// Check for hidden directories
			if entry.Name()[0] == '.' && relPath != "." {
				if !pw.allowAll {
					log.Printf("Skipping hidden directory: %s", relPath)
					continue
				}
			}

			// Queue subdirectory for processing
			select {
			case pw.jobs <- dirJob{path: fullPath}:
				pw.wg.Add(1)
			default:
				// If channel is full, process synchronously
				pw.processDir(fullPath)
			}
		} else {
			// Process file
			pw.processFile(relPath, entry.Name())
		}
	}
}

// processFile processes a single file
func (pw *parallelWalker) processFile(relPath, baseName string) {
	// Skip .ivaldi directory files
	if strings.HasPrefix(relPath, ".ivaldi"+string(filepath.Separator)) || relPath == ".ivaldi" {
		return
	}

	// Check if file is auto-excluded
	if isAutoExcluded(relPath) {
		log.Printf("Auto-excluded for security: %s", relPath)
		return
	}

	// Skip hidden files EXCEPT .ivaldiignore
	if baseName[0] == '.' && relPath != ".ivaldiignore" {
		if !pw.allowAll {
			// For dot files, we need to ask the user (done synchronously via channel)
			pw.dotFileAsks <- relPath
			if <-pw.dotFileResult {
				pw.results <- fileResult{path: relPath}
			}
			return
		}
		fmt.Printf("Warning: Gathering hidden file: %s\n", relPath)
	}

	// Skip ignored files
	if pw.patternCache.IsIgnored(relPath) {
		return
	}

	pw.results <- fileResult{path: relPath}
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

		// Load ignore patterns from .ivaldiignore and create pattern cache
		patternCache, err := ignore.LoadPatternCache(workDir)
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
			// If no arguments, gather all files using parallel walker
			fmt.Println("No files specified, gathering all files in working directory...")

			// Use parallel walker for better performance
			walker := newParallelWalker(workDir, allowAll, patternCache)
			filesToGather = walker.walk()
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
							if patternCache.IsIgnored(relPath) || patternCache.IsIgnored(relPath+"/") {
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
						if patternCache.IsIgnored(relPath) {
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
					if patternCache.IsIgnored(relPath) {
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

