package github

import (
	"runtime"
	"sync"
	"testing"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
)

// BenchmarkParallelHashComputation benchmarks parallel vs sequential hash computation
func BenchmarkParallelHashComputation(b *testing.B) {
	// Generate test data
	fileCount := 500
	files := make(map[string][]byte, fileCount)
	for i := 0; i < fileCount; i++ {
		content := make([]byte, 1024) // 1KB files
		for j := range content {
			content[j] = byte((i + j) % 256)
		}
		files["file"+string(rune('0'+i/100))+string(rune('0'+(i/10)%10))+string(rune('0'+i%10))+".txt"] = content
	}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			hashes := make(map[string]cas.Hash)
			for path, content := range files {
				hashes[path] = cas.SumB3(content)
			}
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		workerCount := runtime.NumCPU()
		if workerCount < 4 {
			workerCount = 4
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var hashes sync.Map
			jobs := make(chan struct {
				path    string
				content []byte
			}, len(files))
			results := make(chan struct {
				path string
				hash cas.Hash
			}, len(files))

			var wg sync.WaitGroup
			for w := 0; w < workerCount; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for job := range jobs {
						results <- struct {
							path string
							hash cas.Hash
						}{job.path, cas.SumB3(job.content)}
					}
				}()
			}

			for path, content := range files {
				jobs <- struct {
					path    string
					content []byte
				}{path, content}
			}
			close(jobs)

			go func() {
				wg.Wait()
				close(results)
			}()

			for result := range results {
				hashes.Store(result.path, result.hash)
			}
		}
	})
}

// BenchmarkDeltaComputation benchmarks change detection
func BenchmarkDeltaComputation(b *testing.B) {
	// Generate test data simulating parent and current files
	fileCount := 500
	parentFiles := make(map[string]cas.Hash, fileCount)
	currentFiles := make(map[string][]byte, fileCount)

	for i := 0; i < fileCount; i++ {
		path := "file" + string(rune('0'+i/100)) + string(rune('0'+(i/10)%10)) + string(rune('0'+i%10)) + ".txt"
		content := make([]byte, 1024)
		for j := range content {
			content[j] = byte((i + j) % 256)
		}

		// 90% unchanged, 5% modified, 5% new
		if i < fileCount*90/100 {
			parentFiles[path] = cas.SumB3(content)
			currentFiles[path] = content
		} else if i < fileCount*95/100 {
			parentFiles[path] = cas.SumB3([]byte("old content"))
			currentFiles[path] = content // Modified
		} else {
			currentFiles[path] = content // New file
		}
	}

	// Add some deleted files
	for i := 0; i < 25; i++ {
		path := "deleted" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + ".txt"
		parentFiles[path] = cas.SumB3([]byte("deleted content"))
	}

	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var changes []FileChange

			// Check for added and modified files
			for filePath, content := range currentFiles {
				currentHash := cas.SumB3(content)
				parentHash, existed := parentFiles[filePath]

				if !existed {
					changes = append(changes, FileChange{Path: filePath, Type: "added"})
				} else if currentHash != parentHash {
					changes = append(changes, FileChange{Path: filePath, Type: "modified"})
				}
			}

			// Check for deleted files
			for filePath := range parentFiles {
				if _, exists := currentFiles[filePath]; !exists {
					changes = append(changes, FileChange{Path: filePath, Type: "deleted"})
				}
			}
			_ = changes
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		workerCount := runtime.NumCPU()
		if workerCount < 4 {
			workerCount = 4
		}

		// Convert parentFiles to sync.Map for parallel access
		var parentMap sync.Map
		for k, v := range parentFiles {
			parentMap.Store(k, v)
		}

		var currentMap sync.Map
		for k, v := range currentFiles {
			currentMap.Store(k, v)
		}

		filePaths := make([]string, 0, len(currentFiles))
		for path := range currentFiles {
			filePaths = append(filePaths, path)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var changes []FileChange
			var changesMu sync.Mutex

			var wg sync.WaitGroup
			jobs := make(chan string, len(filePaths))

			for w := 0; w < workerCount; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for filePath := range jobs {
						contentVal, _ := currentMap.Load(filePath)
						content := contentVal.([]byte)
						currentHash := cas.SumB3(content)
						parentHashVal, existed := parentMap.Load(filePath)

						var change *FileChange
						if !existed {
							change = &FileChange{Path: filePath, Type: "added"}
						} else {
							parentHash := parentHashVal.(cas.Hash)
							if currentHash != parentHash {
								change = &FileChange{Path: filePath, Type: "modified"}
							}
						}

						if change != nil {
							changesMu.Lock()
							changes = append(changes, *change)
							changesMu.Unlock()
						}
					}
				}()
			}

			for _, path := range filePaths {
				jobs <- path
			}
			close(jobs)
			wg.Wait()

			// Check for deleted files
			parentMap.Range(func(key, _ any) bool {
				filePath := key.(string)
				if _, exists := currentMap.Load(filePath); !exists {
					changesMu.Lock()
					changes = append(changes, FileChange{Path: filePath, Type: "deleted"})
					changesMu.Unlock()
				}
				return true
			})
			_ = changes
		}
	})
}

// TestFileChangeTypes ensures FileChange types are correct
func TestFileChangeTypes(t *testing.T) {
	changes := []FileChange{
		{Path: "new.txt", Type: "added"},
		{Path: "modified.txt", Type: "modified"},
		{Path: "deleted.txt", Type: "deleted"},
	}

	expectedTypes := map[string]string{
		"new.txt":      "added",
		"modified.txt": "modified",
		"deleted.txt":  "deleted",
	}

	for _, change := range changes {
		expected, ok := expectedTypes[change.Path]
		if !ok {
			t.Errorf("Unexpected path: %s", change.Path)
			continue
		}
		if change.Type != expected {
			t.Errorf("Path %s: expected type %s, got %s", change.Path, expected, change.Type)
		}
	}
}
