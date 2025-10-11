package converter

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/javanhut/Ivaldi-vcs/internal/keys"
	"github.com/javanhut/Ivaldi-vcs/internal/objects"
	"github.com/javanhut/Ivaldi-vcs/internal/store"
)

// ConversionWorkerPool manages concurrent object conversion
type ConversionWorkerPool struct {
	workers    int
	jobs       chan ConversionJob
	results    chan WorkerResult
	wg         sync.WaitGroup
	db         *store.SharedDB
	objectsDir string
}

// ConversionJob represents a single conversion job
type ConversionJob struct {
	Path    string // File path or object path
	RelPath string // Relative path (for snapshot)
	IsGit   bool   // Whether this is a Git object conversion
}

// WorkerResult represents the result of a single worker conversion
type WorkerResult struct {
	Success bool
	Error   error
}

// NewConversionWorkerPool creates a new worker pool for conversions
func NewConversionWorkerPool(workers int, db *store.SharedDB, objectsDir string) *ConversionWorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers > 8 {
			workers = 8 // Cap at 8 workers
		}
	}

	pool := &ConversionWorkerPool{
		workers:    workers,
		jobs:       make(chan ConversionJob, workers*2),
		results:    make(chan WorkerResult, workers*2),
		db:         db,
		objectsDir: objectsDir,
		wg:         sync.WaitGroup{},
	}

	// Start workers
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker()
	}

	return pool
}

// worker processes conversion jobs
func (pool *ConversionWorkerPool) worker() {
	defer pool.wg.Done()

	for job := range pool.jobs {
		var result WorkerResult

		if job.IsGit {
			result.Error = pool.convertGitObject(job.Path)
		} else {
			result.Error = pool.createBlobFromFile(job.Path, job.RelPath)
		}

		result.Success = result.Error == nil
		pool.results <- result
	}
}

// convertGitObject converts a single Git object
func (pool *ConversionWorkerPool) convertGitObject(objectPath string) error {
	// Convert Git object to Ivaldi format
	digest, content, gitSHA1, err := objects.ConvertGitBlobToIvaldi(objectPath)
	if err != nil {
		return err
	}

	// Generate unique human-readable key
	humanKey, err := keys.GenerateUniquePhrase(pool.db, 3, 4) // 3 words + 4 digits
	if err != nil {
		return fmt.Errorf("generate unique key: %w", err)
	}

	// Store KV mapping (human key -> blake3/sha256)
	if err := pool.db.PutMapping(humanKey, digest.BLAKE3, digest.SHA256); err != nil {
		return fmt.Errorf("store kv mapping: %w", err)
	}

	// Store Git hash mapping if we have one
	if gitSHA1 != "" {
		if err := pool.db.PutGitMapping(gitSHA1, digest.BLAKE3, digest.SHA256); err != nil {
			return fmt.Errorf("store git mapping: %w", err)
		}
	}

	// Store compressed object
	compressed, err := objects.EncodeZstdGitBlob(content)
	if err != nil {
		return fmt.Errorf("compress object: %w", err)
	}

	// Use BLAKE3 hash as filename
	blake3Hex := hex.EncodeToString(digest.BLAKE3[:])
	subDir := filepath.Join(pool.objectsDir, blake3Hex[:2])
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return fmt.Errorf("create object subdir: %w", err)
	}

	objectFile := filepath.Join(subDir, blake3Hex[2:])
	if err := os.WriteFile(objectFile, compressed, 0644); err != nil {
		return fmt.Errorf("write object file: %w", err)
	}

	return nil
}

// createBlobFromFile creates an Ivaldi blob from a file
func (pool *ConversionWorkerPool) createBlobFromFile(filePath, relPath string) error {
	// Read file content
	content, err := objects.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Generate dual hashes
	digest := &objects.DualDigest{
		SHA256: objects.HashBlobSHA256(content),
		BLAKE3: objects.HashBlobBLAKE3(content),
		Size:   len(content),
	}

	// Generate unique human-readable key
	baseKey := filepath.Base(relPath)
	humanKey, err := keys.GenerateUniquePhrase(pool.db, 2, 3) // 2 words + 3 digits
	if err != nil {
		return fmt.Errorf("generate unique key: %w", err)
	}
	humanKey = fmt.Sprintf("%s-%s", baseKey, humanKey)

	// Store KV mapping
	if err := pool.db.PutMapping(humanKey, digest.BLAKE3, digest.SHA256); err != nil {
		return fmt.Errorf("store kv mapping: %w", err)
	}

	// Store compressed object
	compressed, err := objects.EncodeZstdGitBlob(content)
	if err != nil {
		return fmt.Errorf("compress object: %w", err)
	}

	// Use BLAKE3 hash as filename
	blake3Hex := hex.EncodeToString(digest.BLAKE3[:])
	subDir := filepath.Join(pool.objectsDir, blake3Hex[:2])
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return fmt.Errorf("create object subdir: %w", err)
	}

	objectFile := filepath.Join(subDir, blake3Hex[2:])
	if err := os.WriteFile(objectFile, compressed, 0644); err != nil {
		return fmt.Errorf("write object file: %w", err)
	}

	return nil
}

// Submit submits a job to the pool
func (pool *ConversionWorkerPool) Submit(job ConversionJob) {
	pool.jobs <- job
}

// Close shuts down the worker pool and waits for completion
func (pool *ConversionWorkerPool) Close() (*ConversionResult, error) {
	close(pool.jobs)

	// Wait for all workers to finish
	pool.wg.Wait()
	close(pool.results)

	// Collect results
	result := &ConversionResult{}
	for r := range pool.results {
		if r.Success {
			result.Converted++
		} else {
			result.Skipped++
			if r.Error != nil {
				result.Errors = append(result.Errors, r.Error)
			}
		}
	}

	return result, nil
}

// ConvertGitObjectsToIvaldiConcurrent discovers and converts Git objects using concurrent workers
func ConvertGitObjectsToIvaldiConcurrent(gitDir, ivaldiDir string, workers int) (*ConversionResult, error) {
	// Open the Ivaldi KV store
	fmt.Println("Opening shared Ivaldi KV store for Git conversion...")
	db, err := store.GetSharedDB(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("open shared ivaldi store: %w", err)
	}
	defer db.Close()
	fmt.Println("Shared KV store opened successfully")

	// Create objects directory
	objectsDir := filepath.Join(ivaldiDir, "objects")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		return nil, fmt.Errorf("create objects dir: %w", err)
	}

	// Discover Git objects
	objectPaths, err := objects.DiscoverGitObjects(gitDir)
	if err != nil {
		return nil, fmt.Errorf("discover git objects: %w", err)
	}

	fmt.Printf("Found %d Git objects to convert\n", len(objectPaths))

	// Create worker pool
	pool := NewConversionWorkerPool(workers, db, objectsDir)

	// Start result collector goroutine to prevent deadlock
	// This drains the results channel while jobs are being submitted
	resultsChan := make(chan *ConversionResult, 1)
	jobsSubmitted := len(objectPaths)

	go func() {
		result := &ConversionResult{}
		for i := 0; i < jobsSubmitted; i++ {
			r := <-pool.results
			if r.Success {
				result.Converted++
			} else {
				result.Skipped++
				if r.Error != nil {
					result.Errors = append(result.Errors, r.Error)
				}
			}
		}
		resultsChan <- result
	}()

	// Submit all jobs
	fmt.Printf("Converting Git objects using %d workers...\n", workers)
	for i, objectPath := range objectPaths {
		fmt.Printf("Submitting Git object %d/%d: %s\n", i+1, len(objectPaths), objectPath)
		pool.Submit(ConversionJob{
			Path:  objectPath,
			IsGit: true,
		})
	}

	// Close jobs channel and wait for workers to finish
	close(pool.jobs)
	pool.wg.Wait()

	// Get collected results from result collector goroutine
	result := <-resultsChan

	return result, nil
}

// SnapshotCurrentFilesConcurrent creates blob objects for all files using concurrent workers
func SnapshotCurrentFilesConcurrent(workDir, ivaldiDir string, workers int) (*ConversionResult, error) {
	// Open the Ivaldi KV store
	db, err := store.GetSharedDB(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("open shared ivaldi store: %w", err)
	}
	defer db.Close()

	// Create objects directory
	objectsDir := filepath.Join(ivaldiDir, "objects")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		return nil, fmt.Errorf("create objects dir: %w", err)
	}

	// Collect all files to snapshot
	var files []struct {
		Path    string
		RelPath string
	}

	err = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
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

		// Skip .git and .ivaldi directories
		if strings.HasPrefix(relPath, ".git") || strings.HasPrefix(relPath, ".ivaldi") {
			return nil
		}

		files = append(files, struct {
			Path    string
			RelPath string
		}{Path: path, RelPath: relPath})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk working directory: %w", err)
	}

	fmt.Printf("Found %d files to snapshot\n", len(files))

	// Create worker pool
	pool := NewConversionWorkerPool(workers, db, objectsDir)

	// Start result collector goroutine to prevent deadlock
	// This drains the results channel while jobs are being submitted
	resultsChan := make(chan *ConversionResult, 1)
	jobsSubmitted := len(files)

	go func() {
		result := &ConversionResult{}
		for i := 0; i < jobsSubmitted; i++ {
			r := <-pool.results
			if r.Success {
				result.Converted++
			} else {
				result.Skipped++
				if r.Error != nil {
					result.Errors = append(result.Errors, r.Error)
				}
			}
		}
		resultsChan <- result
	}()

	// Submit all jobs
	fmt.Printf("Snapshotting files using %d workers...\n", workers)
	for _, file := range files {
		fmt.Printf("Snapshotting file: %s\n", file.RelPath)
		pool.Submit(ConversionJob{
			Path:    file.Path,
			RelPath: file.RelPath,
			IsGit:   false,
		})
	}

	// Close jobs channel and wait for workers to finish
	close(pool.jobs)
	pool.wg.Wait()

	// Get collected results from result collector goroutine
	result := <-resultsChan

	return result, nil
}
