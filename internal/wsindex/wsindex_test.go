package wsindex

import (
	"fmt"
	"testing"
	"time"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/filechunk"
)

func createTestFile(path, content string) FileMetadata {
	contentBytes := []byte(content)
	hash := cas.SumB3(contentBytes)
	
	return FileMetadata{
		Path: path,
		FileRef: filechunk.NodeRef{
			Hash: hash,
			Kind: filechunk.Leaf,
			Size: int64(len(contentBytes)),
		},
		ModTime:  time.Unix(1640995200, 0), // 2022-01-01
		Mode:     0644,
		Size:     int64(len(contentBytes)),
		Checksum: hash,
	}
}

func TestEmptyIndex(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	index, err := builder.Build(nil)
	if err != nil {
		t.Fatalf("Build empty index failed: %v", err)
	}
	
	if index.Count != 0 {
		t.Errorf("Expected empty index count 0, got %d", index.Count)
	}
	
	loader := NewLoader(casStore)
	files, err := loader.ListAll(index)
	if err != nil {
		t.Fatalf("ListAll empty index failed: %v", err)
	}
	
	if len(files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(files))
	}
}

func TestSingleFile(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	testFile := createTestFile("test.txt", "hello world")
	files := []FileMetadata{testFile}
	
	index, err := builder.Build(files)
	if err != nil {
		t.Fatalf("Build index failed: %v", err)
	}
	
	if index.Count != 1 {
		t.Errorf("Expected index count 1, got %d", index.Count)
	}
	
	loader := NewLoader(casStore)
	
	// Test lookup
	found, err := loader.Lookup(index, "test.txt")
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}
	if found == nil {
		t.Fatal("File not found")
	}
	if found.Path != "test.txt" {
		t.Errorf("Path mismatch: expected test.txt, got %s", found.Path)
	}
	if found.FileRef.Hash != testFile.FileRef.Hash {
		t.Error("File hash mismatch")
	}
	
	// Test lookup non-existent
	found, err = loader.Lookup(index, "nonexistent.txt")
	if err != nil {
		t.Fatalf("Lookup non-existent failed: %v", err)
	}
	if found != nil {
		t.Error("Expected nil for non-existent file")
	}
	
	// Test list all
	allFiles, err := loader.ListAll(index)
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}
	if len(allFiles) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(allFiles))
	}
	if allFiles[0].Path != "test.txt" {
		t.Errorf("Listed file path mismatch: got %s", allFiles[0].Path)
	}
}

func TestMultipleFiles(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	files := []FileMetadata{
		createTestFile("file1.txt", "content1"),
		createTestFile("file2.txt", "content2"),
		createTestFile("dir/file3.txt", "content3"),
		createTestFile("dir/subdir/file4.txt", "content4"),
	}
	
	index, err := builder.Build(files)
	if err != nil {
		t.Fatalf("Build index failed: %v", err)
	}
	
	if index.Count != 4 {
		t.Errorf("Expected index count 4, got %d", index.Count)
	}
	
	loader := NewLoader(casStore)
	
	// Test lookup each file
	for _, expectedFile := range files {
		found, err := loader.Lookup(index, expectedFile.Path)
		if err != nil {
			t.Fatalf("Lookup %s failed: %v", expectedFile.Path, err)
		}
		if found == nil {
			t.Fatalf("File %s not found", expectedFile.Path)
		}
		if found.Path != expectedFile.Path {
			t.Errorf("Path mismatch: expected %s, got %s", expectedFile.Path, found.Path)
		}
		if found.FileRef.Hash != expectedFile.FileRef.Hash {
			t.Errorf("Hash mismatch for %s", expectedFile.Path)
		}
	}
	
	// Test list all
	allFiles, err := loader.ListAll(index)
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}
	if len(allFiles) != 4 {
		t.Fatalf("Expected 4 files, got %d", len(allFiles))
	}
	
	// Check that files are sorted by path
	for i := 1; i < len(allFiles); i++ {
		if allFiles[i-1].Path >= allFiles[i].Path {
			t.Errorf("Files not sorted: %s >= %s", allFiles[i-1].Path, allFiles[i].Path)
		}
	}
}

func TestPrefixListing(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	files := []FileMetadata{
		createTestFile("dir/file1.txt", "content1"),
		createTestFile("dir/file2.txt", "content2"),
		createTestFile("dir/subdir/file3.txt", "content3"),
		createTestFile("other/file4.txt", "content4"),
		createTestFile("root.txt", "root content"),
	}
	
	index, err := builder.Build(files)
	if err != nil {
		t.Fatalf("Build index failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	// Test prefix "dir/"
	dirFiles, err := loader.ListPrefix(index, "dir/")
	if err != nil {
		t.Fatalf("ListPrefix failed: %v", err)
	}
	if len(dirFiles) != 3 {
		t.Fatalf("Expected 3 files with prefix 'dir/', got %d", len(dirFiles))
	}
	
	expectedPaths := []string{"dir/file1.txt", "dir/file2.txt", "dir/subdir/file3.txt"}
	for i, file := range dirFiles {
		if file.Path != expectedPaths[i] {
			t.Errorf("Prefix result %d: expected %s, got %s", i, expectedPaths[i], file.Path)
		}
	}
	
	// Test prefix "dir/subdir/"
	subdirFiles, err := loader.ListPrefix(index, "dir/subdir/")
	if err != nil {
		t.Fatalf("ListPrefix subdir failed: %v", err)
	}
	if len(subdirFiles) != 1 {
		t.Fatalf("Expected 1 file with prefix 'dir/subdir/', got %d", len(subdirFiles))
	}
	if subdirFiles[0].Path != "dir/subdir/file3.txt" {
		t.Errorf("Subdir prefix result: expected dir/subdir/file3.txt, got %s", subdirFiles[0].Path)
	}
	
	// Test non-matching prefix
	noFiles, err := loader.ListPrefix(index, "nonexistent/")
	if err != nil {
		t.Fatalf("ListPrefix non-existent failed: %v", err)
	}
	if len(noFiles) != 0 {
		t.Errorf("Expected 0 files with non-matching prefix, got %d", len(noFiles))
	}
}

func TestRangeListing(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	files := []FileMetadata{
		createTestFile("a/file1.txt", "content1"),
		createTestFile("b/file2.txt", "content2"),
		createTestFile("c/file3.txt", "content3"),
		createTestFile("d/file4.txt", "content4"),
		createTestFile("e/file5.txt", "content5"),
	}
	
	index, err := builder.Build(files)
	if err != nil {
		t.Fatalf("Build index failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	// Test range ["b/", "d/") - should include b/ and c/ but not d/
	rangeFiles, err := loader.ListRange(index, "b/", "d/")
	if err != nil {
		t.Fatalf("ListRange failed: %v", err)
	}
	if len(rangeFiles) != 2 {
		t.Fatalf("Expected 2 files in range [b/, d/), got %d", len(rangeFiles))
	}
	
	expectedPaths := []string{"b/file2.txt", "c/file3.txt"}
	for i, file := range rangeFiles {
		if file.Path != expectedPaths[i] {
			t.Errorf("Range result %d: expected %s, got %s", i, expectedPaths[i], file.Path)
		}
	}
}

func TestWalk(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	files := []FileMetadata{
		createTestFile("file1.txt", "content1"),
		createTestFile("file2.txt", "content2"),
		createTestFile("dir/file3.txt", "content3"),
	}
	
	index, err := builder.Build(files)
	if err != nil {
		t.Fatalf("Build index failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	var walked []string
	err = loader.Walk(index, func(file FileMetadata) error {
		walked = append(walked, file.Path)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}
	
	if len(walked) != 3 {
		t.Fatalf("Expected 3 files walked, got %d", len(walked))
	}
	
	// Should be in sorted order
	expectedOrder := []string{"dir/file3.txt", "file1.txt", "file2.txt"}
	for i, path := range walked {
		if path != expectedOrder[i] {
			t.Errorf("Walk order %d: expected %s, got %s", i, expectedOrder[i], path)
		}
	}
}

func TestLargeIndex(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	builder.LeafSize = 16 // Small leaf size to force internal nodes
	
	// Create a large index to test tree structure
	var files []FileMetadata
	for i := 0; i < 200; i++ {
		path := fmt.Sprintf("dir%02d/file%03d.txt", i/10, i)
		content := fmt.Sprintf("content for file %d", i)
		files = append(files, createTestFile(path, content))
	}
	
	index, err := builder.Build(files)
	if err != nil {
		t.Fatalf("Build large index failed: %v", err)
	}
	
	if index.Count != 200 {
		t.Errorf("Expected index count 200, got %d", index.Count)
	}
	
	loader := NewLoader(casStore)
	
	// Test lookup of each file
	for _, expectedFile := range files {
		found, err := loader.Lookup(index, expectedFile.Path)
		if err != nil {
			t.Fatalf("Lookup %s failed: %v", expectedFile.Path, err)
		}
		if found == nil {
			t.Fatalf("File %s not found", expectedFile.Path)
		}
		if found.Path != expectedFile.Path {
			t.Errorf("Path mismatch for %s", expectedFile.Path)
		}
	}
	
	// Test list all
	allFiles, err := loader.ListAll(index)
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}
	if len(allFiles) != 200 {
		t.Errorf("Expected 200 files listed, got %d", len(allFiles))
	}
}

func TestDiff(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	loader := NewLoader(casStore)
	
	// Create old index
	oldFiles := []FileMetadata{
		createTestFile("file1.txt", "old content 1"),
		createTestFile("file2.txt", "content 2"),
		createTestFile("file3.txt", "content 3"),
	}
	
	oldIndex, err := builder.Build(oldFiles)
	if err != nil {
		t.Fatalf("Build old index failed: %v", err)
	}
	
	// Create new index with changes
	newFiles := []FileMetadata{
		createTestFile("file1.txt", "new content 1"), // Modified
		createTestFile("file2.txt", "content 2"),      // Unchanged
		createTestFile("file4.txt", "content 4"),      // Added
		// file3.txt removed
	}
	
	newIndex, err := builder.Build(newFiles)
	if err != nil {
		t.Fatalf("Build new index failed: %v", err)
	}
	
	// Compute diff
	diff, err := loader.Diff(oldIndex, newIndex)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	
	// Check added files
	if len(diff.Added) != 1 {
		t.Errorf("Expected 1 added file, got %d", len(diff.Added))
	} else if diff.Added[0].Path != "file4.txt" {
		t.Errorf("Added file: expected file4.txt, got %s", diff.Added[0].Path)
	}
	
	// Check modified files
	if len(diff.Modified) != 1 {
		t.Errorf("Expected 1 modified file, got %d", len(diff.Modified))
	} else if diff.Modified[0].Path != "file1.txt" {
		t.Errorf("Modified file: expected file1.txt, got %s", diff.Modified[0].Path)
	}
	
	// Check removed files
	if len(diff.Removed) != 1 {
		t.Errorf("Expected 1 removed file, got %d", len(diff.Removed))
	} else if diff.Removed[0].Path != "file3.txt" {
		t.Errorf("Removed file: expected file3.txt, got %s", diff.Removed[0].Path)
	}
}

func TestSameContentSameHash(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	files := []FileMetadata{
		createTestFile("test.txt", "content"),
	}
	
	index1, err := builder.Build(files)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	
	index2, err := builder.Build(files)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}
	
	if index1.Hash != index2.Hash {
		t.Error("Same index content should produce same hash")
	}
}

func TestDifferentContentDifferentHash(t *testing.T) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	files1 := []FileMetadata{
		createTestFile("test1.txt", "content1"),
	}
	
	files2 := []FileMetadata{
		createTestFile("test2.txt", "content2"),
	}
	
	index1, err := builder.Build(files1)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	
	index2, err := builder.Build(files2)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}
	
	if index1.Hash == index2.Hash {
		t.Error("Different index content should produce different hashes")
	}
}

func BenchmarkBuildSmallIndex(b *testing.B) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	files := []FileMetadata{
		createTestFile("file1.txt", "content1"),
		createTestFile("file2.txt", "content2"),
		createTestFile("dir/file3.txt", "content3"),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.Build(files)
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
	}
}

func BenchmarkLookupLargeIndex(b *testing.B) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Build large index
	var files []FileMetadata
	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("dir%02d/file%03d.txt", i/50, i)
		content := fmt.Sprintf("content%d", i)
		files = append(files, createTestFile(path, content))
	}
	
	index, err := builder.Build(files)
	if err != nil {
		b.Fatalf("Build failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := fmt.Sprintf("dir%02d/file%03d.txt", (i%1000)/50, i%1000)
		_, err := loader.Lookup(index, path)
		if err != nil {
			b.Fatalf("Lookup failed: %v", err)
		}
	}
}

func BenchmarkListAllLargeIndex(b *testing.B) {
	casStore := cas.NewMemoryCAS()
	builder := NewBuilder(casStore)
	
	// Build large index
	var files []FileMetadata
	for i := 0; i < 1000; i++ {
		path := fmt.Sprintf("file%04d.txt", i)
		content := fmt.Sprintf("content%d", i)
		files = append(files, createTestFile(path, content))
	}
	
	index, err := builder.Build(files)
	if err != nil {
		b.Fatalf("Build failed: %v", err)
	}
	
	loader := NewLoader(casStore)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := loader.ListAll(index)
		if err != nil {
			b.Fatalf("ListAll failed: %v", err)
		}
	}
}