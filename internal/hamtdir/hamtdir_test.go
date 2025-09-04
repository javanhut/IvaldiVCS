package hamtdir

import (
	"fmt"
	"testing"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
)

func TestEmptyDirectory(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	dir, err := builder.Build(nil)
	if err != nil {
		t.Fatalf("Build empty directory failed: %v", err)
	}
	
	if dir.Size != 0 {
		t.Errorf("Expected empty directory size 0, got %d", dir.Size)
	}
	
	loader := NewLoader(casStore)
	entries, err := loader.List(dir)
	if err != nil {
		t.Fatalf("List empty directory failed: %v", err)
	}
	
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(entries))
	}
}

func TestSingleFileDirectory(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Create a file entry
	fileRef := &filechunk.NodeRef{
		Hash: cas.SumB3([]byte("test content")),
		Kind: filechunk.Leaf,
		Size: 12,
	}
	
	entries := []Entry{
		{
			Name: "test.txt",
			Type: FileEntry,
			File: fileRef,
		},
	}
	
	dir, err := builder.Build(entries)
	if err != nil {
		t.Fatalf("Build directory failed: %v", err)
	}
	
	if dir.Size != 1 {
		t.Errorf("Expected directory size 1, got %d", dir.Size)
	}
	
	loader := NewLoader(casStore)
	
	// Test lookup
	entry, err := loader.Lookup(dir, "test.txt")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if entry == nil {
		t.Fatal("Entry not found")
	}
	if entry.Name != "test.txt" || entry.Type != FileEntry {
		t.Errorf("Entry mismatch: got %+v", entry)
	}
	if entry.File.Hash != fileRef.Hash {
		t.Errorf("File hash mismatch")
	}
	
	// Test lookup non-existent
	entry, err = loader.Lookup(dir, "nonexistent.txt")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if entry != nil {
		t.Error("Expected nil for non-existent entry")
	}
	
	// Test list
	listed, err := loader.List(dir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(listed))
	}
	if listed[0].Name != "test.txt" {
		t.Errorf("Listed entry name mismatch: got %s", listed[0].Name)
	}
}

func TestMultipleFilesDirectory(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Create multiple file entries
	entries := []Entry{
		{
			Name: "file1.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content1")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
		{
			Name: "file2.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content2")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
		{
			Name: "file3.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content3")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
	}
	
	dir, err := builder.Build(entries)
	if err != nil {
		t.Fatalf("Build directory failed: %v", err)
	}
	
	if dir.Size != 3 {
		t.Errorf("Expected directory size 3, got %d", dir.Size)
	}
	
	loader := NewLoader(casStore)
	
	// Test lookup each file
	for _, expectedEntry := range entries {
		entry, err := loader.Lookup(dir, expectedEntry.Name)
		if err != nil {
			t.Fatalf("Lookup %s failed: %v", expectedEntry.Name, err)
		}
		if entry == nil {
			t.Fatalf("Entry %s not found", expectedEntry.Name)
		}
		if entry.Name != expectedEntry.Name {
			t.Errorf("Entry name mismatch: expected %s, got %s", expectedEntry.Name, entry.Name)
		}
		if entry.File.Hash != expectedEntry.File.Hash {
			t.Errorf("File hash mismatch for %s", expectedEntry.Name)
		}
	}
	
	// Test list all
	listed, err := loader.List(dir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(listed))
	}
}

func TestNestedDirectory(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Build subdirectory first
	subEntries := []Entry{
		{
			Name: "subfile.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("sub content")),
				Kind: filechunk.Leaf,
				Size: 11,
			},
		},
	}
	
	subDir, err := builder.Build(subEntries)
	if err != nil {
		t.Fatalf("Build subdirectory failed: %v", err)
	}
	
	// Build main directory with subdirectory
	entries := []Entry{
		{
			Name: "file.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("main content")),
				Kind: filechunk.Leaf,
				Size: 12,
			},
		},
		{
			Name: "subdir",
			Type: DirEntry,
			Dir:  &subDir,
		},
	}
	
	mainDir, err := builder.Build(entries)
	if err != nil {
		t.Fatalf("Build main directory failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	// Test lookup file in main directory
	entry, err := loader.Lookup(mainDir, "file.txt")
	if err != nil {
		t.Fatalf("Lookup file.txt failed: %v", err)
	}
	if entry == nil || entry.Name != "file.txt" {
		t.Error("file.txt not found or incorrect")
	}
	
	// Test lookup subdirectory
	subDirEntry, err := loader.Lookup(mainDir, "subdir")
	if err != nil {
		t.Fatalf("Lookup subdir failed: %v", err)
	}
	if subDirEntry == nil || subDirEntry.Type != DirEntry {
		t.Error("subdir not found or not a directory")
	}
	
	// Test lookup file in subdirectory
	subFileEntry, err := loader.Lookup(*subDirEntry.Dir, "subfile.txt")
	if err != nil {
		t.Fatalf("Lookup subfile.txt failed: %v", err)
	}
	if subFileEntry == nil || subFileEntry.Name != "subfile.txt" {
		t.Error("subfile.txt not found or incorrect")
	}
}

func TestPathLookup(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Build nested directory structure
	// subdir/
	//   deep/
	//     file.txt
	//   other.txt
	// main.txt
	
	deepFileEntry := Entry{
		Name: "file.txt",
		Type: FileEntry,
		File: &filechunk.NodeRef{
			Hash: cas.SumB3([]byte("deep content")),
			Kind: filechunk.Leaf,
			Size: 12,
		},
	}
	
	deepDir, err := builder.Build([]Entry{deepFileEntry})
	if err != nil {
		t.Fatalf("Build deep directory failed: %v", err)
	}
	
	subDirEntries := []Entry{
		{
			Name: "deep",
			Type: DirEntry,
			Dir:  &deepDir,
		},
		{
			Name: "other.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("other content")),
				Kind: filechunk.Leaf,
				Size: 13,
			},
		},
	}
	
	subDir, err := builder.Build(subDirEntries)
	if err != nil {
		t.Fatalf("Build subdirectory failed: %v", err)
	}
	
	mainEntries := []Entry{
		{
			Name: "subdir",
			Type: DirEntry,
			Dir:  &subDir,
		},
		{
			Name: "main.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("main content")),
				Kind: filechunk.Leaf,
				Size: 12,
			},
		},
	}
	
	rootDir, err := builder.Build(mainEntries)
	if err != nil {
		t.Fatalf("Build root directory failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	// Test various path lookups
	tests := []struct {
		path     string
		expected string
		exists   bool
	}{
		{"main.txt", "main.txt", true},
		{"subdir/other.txt", "other.txt", true},
		{"subdir/deep/file.txt", "file.txt", true},
		{"nonexistent.txt", "", false},
		{"subdir/nonexistent.txt", "", false},
		{"subdir/deep/nonexistent.txt", "", false},
	}
	
	for _, test := range tests {
		entry, err := loader.PathLookup(rootDir, test.path)
		if err != nil {
			t.Fatalf("PathLookup %s failed: %v", test.path, err)
		}
		
		if test.exists {
			if entry == nil {
				t.Errorf("Expected to find %s", test.path)
			} else if entry.Name != test.expected {
				t.Errorf("Path %s: expected name %s, got %s", test.path, test.expected, entry.Name)
			}
		} else {
			if entry != nil {
				t.Errorf("Expected %s to not exist, but found %+v", test.path, entry)
			}
		}
	}
}

func TestWalkEntries(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Build simple directory structure
	entries := []Entry{
		{
			Name: "file1.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content1")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
		{
			Name: "file2.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content2")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
	}
	
	dir, err := builder.Build(entries)
	if err != nil {
		t.Fatalf("Build directory failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	var walked []string
	err = loader.WalkEntries(dir, func(path string, entry Entry) error {
		walked = append(walked, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkEntries failed: %v", err)
	}
	
	if len(walked) != 2 {
		t.Fatalf("Expected 2 entries walked, got %d: %v", len(walked), walked)
	}
	
	// Check that all files were walked (order might vary)
	found := make(map[string]bool)
	for _, path := range walked {
		found[path] = true
	}
	
	if !found["file1.txt"] || !found["file2.txt"] {
		t.Errorf("Missing expected files in walk result: %v", walked)
	}
}

func TestLargeDirectory(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Create a large directory to test HAMT internal node creation
	var entries []Entry
	for i := 0; i < 100; i++ {
		filename := fmt.Sprintf("file%03d.txt", i)
		content := fmt.Sprintf("content of file %d", i)
		
		entries = append(entries, Entry{
			Name: filename,
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte(content)),
				Kind: filechunk.Leaf,
				Size: int64(len(content)),
			},
		})
	}
	
	dir, err := builder.Build(entries)
	if err != nil {
		t.Fatalf("Build large directory failed: %v", err)
	}
	
	if dir.Size != 100 {
		t.Errorf("Expected directory size 100, got %d", dir.Size)
	}
	
	loader := NewLoader(casStore)
	
	// Test lookup of each file
	for i := 0; i < 100; i++ {
		filename := fmt.Sprintf("file%03d.txt", i)
		entry, err := loader.Lookup(dir, filename)
		if err != nil {
			t.Fatalf("Lookup %s failed: %v", filename, err)
		}
		if entry == nil {
			t.Fatalf("File %s not found", filename)
		}
		if entry.Name != filename {
			t.Errorf("File name mismatch: expected %s, got %s", filename, entry.Name)
		}
	}
	
	// Test list all
	listed, err := loader.List(dir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listed) != 100 {
		t.Errorf("Expected 100 entries listed, got %d", len(listed))
	}
}

func TestSameContentSameHash(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	entries := []Entry{
		{
			Name: "test.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content")),
				Kind: filechunk.Leaf,
				Size: 7,
			},
		},
	}
	
	dir1, err := builder.Build(entries)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	
	dir2, err := builder.Build(entries)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}
	
	if dir1.Hash != dir2.Hash {
		t.Error("Same directory content should produce same hash")
	}
}

func TestDifferentContentDifferentHash(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	entries1 := []Entry{
		{
			Name: "test1.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content1")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
	}
	
	entries2 := []Entry{
		{
			Name: "test2.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content2")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
	}
	
	dir1, err := builder.Build(entries1)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	
	dir2, err := builder.Build(entries2)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}
	
	if dir1.Hash == dir2.Hash {
		t.Error("Different directory content should produce different hashes")
	}
}

func BenchmarkBuildSmallDir(b *testing.B) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	entries := []Entry{
		{
			Name: "file1.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content1")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
		{
			Name: "file2.txt",
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte("content2")),
				Kind: filechunk.Leaf,
				Size: 8,
			},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.Build(entries)
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
	}
}

func BenchmarkLookupLargeDir(b *testing.B) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Build large directory
	var entries []Entry
	for i := 0; i < 1000; i++ {
		filename := fmt.Sprintf("file%04d.txt", i)
		content := fmt.Sprintf("content%d", i)
		
		entries = append(entries, Entry{
			Name: filename,
			Type: FileEntry,
			File: &filechunk.NodeRef{
				Hash: cas.SumB3([]byte(content)),
				Kind: filechunk.Leaf,
				Size: int64(len(content)),
			},
		})
	}
	
	dir, err := builder.Build(entries)
	if err != nil {
		b.Fatalf("Build failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filename := fmt.Sprintf("file%04d.txt", i%1000)
		_, err := loader.Lookup(dir, filename)
		if err != nil {
			b.Fatalf("Lookup failed: %v", err)
		}
	}
}