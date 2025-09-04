package fsmerkle

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkBuildTreeFromMap(b *testing.B) {
	sizes := []int{10, 100, 1000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("N=%d", size), func(b *testing.B) {
			files := generateTestFiles(size)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _, err := BuildTreeFromMap(files)
				if err != nil {
					b.Fatalf("BuildTreeFromMap failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkBlobHashing(b *testing.B) {
	sizes := []int{1024, 10240, 102400} // 1KB, 10KB, 100KB
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size=%dB", size), func(b *testing.B) {
			content := make([]byte, size)
			for i := range content {
				content[i] = byte(i % 256)
			}
			
			blob := &BlobNode{Size: len(content)}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = blob.Hash(content)
			}
			b.SetBytes(int64(size))
		})
	}
}

func BenchmarkTreeHashing(b *testing.B) {
	entryCount := []int{10, 100, 1000}
	
	for _, count := range entryCount {
		b.Run(fmt.Sprintf("Entries=%d", count), func(b *testing.B) {
			entries := make([]Entry, count)
			for i := 0; i < count; i++ {
				entries[i] = Entry{
					Name: fmt.Sprintf("file%06d.txt", i),
					Mode: 0100644,
					Kind: KindBlob,
					Hash: [32]byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)},
				}
			}
			
			tree := &TreeNode{Entries: entries}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := tree.Hash()
				if err != nil {
					b.Fatalf("Tree hash failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkDiffTrees(b *testing.B) {
	cas := NewMemoryCAS()
	store := NewStore(cas)
	
	// Create base tree with many files
	baseFiles := generateTestFiles(1000)
	baseTree := createTreeFromFiles(b, store, baseFiles)
	
	// Create modified tree (change 1% of files)
	modifiedFiles := make(map[string][]byte)
	for k, v := range baseFiles {
		modifiedFiles[k] = v
	}
	
	changeCount := 10
	i := 0
	for path := range modifiedFiles {
		if i >= changeCount {
			break
		}
		modifiedFiles[path] = []byte(fmt.Sprintf("modified content %d", i))
		i++
	}
	
	modifiedTree := createTreeFromFiles(b, store, modifiedFiles)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		changes, err := DiffTrees(baseTree, modifiedTree, store)
		if err != nil {
			b.Fatalf("DiffTrees failed: %v", err)
		}
		_ = changes
	}
}

func BenchmarkDiffTreesIdentical(b *testing.B) {
	cas := NewMemoryCAS()
	store := NewStore(cas)
	
	// Create tree with many files
	files := generateTestFiles(1000)
	treeHash := createTreeFromFiles(b, store, files)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		changes, err := DiffTrees(treeHash, treeHash, store)
		if err != nil {
			b.Fatalf("DiffTrees failed: %v", err)
		}
		if len(changes) != 0 {
			b.Fatalf("Expected no changes, got %d", len(changes))
		}
	}
}

func BenchmarkStoreOperations(b *testing.B) {
	b.Run("PutBlob", func(b *testing.B) {
		cas := NewMemoryCAS()
		store := NewStore(cas)
		content := []byte("benchmark content")
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Use different content each time to avoid hash collisions
			testContent := append(content, byte(i%256))
			_, _, err := store.PutBlob(testContent)
			if err != nil {
				b.Fatalf("PutBlob failed: %v", err)
			}
		}
	})
	
	b.Run("LoadBlob", func(b *testing.B) {
		cas := NewMemoryCAS()
		store := NewStore(cas)
		content := []byte("benchmark content")
		hash, _, err := store.PutBlob(content)
		if err != nil {
			b.Fatalf("PutBlob failed: %v", err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, err := store.LoadBlob(hash)
			if err != nil {
				b.Fatalf("LoadBlob failed: %v", err)
			}
		}
	})
	
	b.Run("PutTree", func(b *testing.B) {
		cas := NewMemoryCAS()
		store := NewStore(cas)
		
		entries := []Entry{
			{Name: "file1.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
			{Name: "file2.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{2}},
			{Name: "subdir", Mode: 040000, Kind: KindTree, Hash: [32]byte{3}},
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Modify an entry to create different trees
			entries[0].Hash = [32]byte{byte(i % 256)}
			_, err := store.PutTree(entries)
			if err != nil {
				b.Fatalf("PutTree failed: %v", err)
			}
		}
	})
	
	b.Run("LoadTree", func(b *testing.B) {
		cas := NewMemoryCAS()
		store := NewStore(cas)
		
		entries := []Entry{
			{Name: "file1.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{1}},
			{Name: "file2.txt", Mode: 0100644, Kind: KindBlob, Hash: [32]byte{2}},
		}
		
		hash, err := store.PutTree(entries)
		if err != nil {
			b.Fatalf("PutTree failed: %v", err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := store.LoadTree(hash)
			if err != nil {
				b.Fatalf("LoadTree failed: %v", err)
			}
		}
	})
}

// Helper functions for benchmarks

func generateTestFiles(count int) map[string][]byte {
	files := make(map[string][]byte, count)
	
	for i := 0; i < count; i++ {
		// Create a mix of flat and nested files
		var path string
		if i%10 == 0 {
			// 10% in subdirectories
			path = fmt.Sprintf("subdir%d/file%06d.txt", i/100, i)
		} else {
			// 90% in root
			path = fmt.Sprintf("file%06d.txt", i)
		}
		
		files[path] = []byte(fmt.Sprintf("Content for file %d\nThis is line 2\nLine 3 has more content", i))
	}
	
	return files
}

func createTreeFromFiles(b *testing.B, store *Store, files map[string][]byte) Hash {
	b.Helper()
	
	// This is a simplified version - in practice you'd need to implement
	// the full tree building logic that BuildTreeFromMap uses but with
	// a provided store
	
	// For this benchmark, we'll create a simple flat tree
	entries := make([]Entry, 0, len(files))
	
	for path, content := range files {
		hash, _, err := store.PutBlob(content)
		if err != nil {
			b.Fatalf("PutBlob failed: %v", err)
		}
		
		// Simplified: just use filename, no directories
		filename := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			filename = path[idx+1:]
		}
		
		entries = append(entries, Entry{
			Name: filename,
			Mode: 0100644,
			Kind: KindBlob,
			Hash: hash,
		})
	}
	
	hash, err := store.PutTree(entries)
	if err != nil {
		b.Fatalf("PutTree failed: %v", err)
	}
	
	return hash
}

// Benchmark canonical encoding operations
func BenchmarkCanonicalEncoding(b *testing.B) {
	b.Run("BlobCanonical", func(b *testing.B) {
		blob := &BlobNode{Size: 1024}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = blob.CanonicalBytes()
		}
	})
	
	b.Run("TreeCanonical", func(b *testing.B) {
		entries := make([]Entry, 100)
		for i := 0; i < 100; i++ {
			entries[i] = Entry{
				Name: fmt.Sprintf("file%03d.txt", i),
				Mode: 0100644,
				Kind: KindBlob,
				Hash: [32]byte{byte(i)},
			}
		}
		
		tree := &TreeNode{Entries: entries}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := tree.CanonicalBytes()
			if err != nil {
				b.Fatalf("CanonicalBytes failed: %v", err)
			}
		}
	})
	
	b.Run("TreeParsing", func(b *testing.B) {
		entries := make([]Entry, 100)
		for i := 0; i < 100; i++ {
			entries[i] = Entry{
				Name: fmt.Sprintf("file%03d.txt", i),
				Mode: 0100644,
				Kind: KindBlob,
				Hash: [32]byte{byte(i)},
			}
		}
		
		tree := &TreeNode{Entries: entries}
		canonical, err := tree.CanonicalBytes()
		if err != nil {
			b.Fatalf("CanonicalBytes failed: %v", err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := parseTreeCanonical(canonical)
			if err != nil {
				b.Fatalf("parseTreeCanonical failed: %v", err)
			}
		}
	})
}