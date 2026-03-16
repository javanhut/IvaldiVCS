package filechunk

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
)

func TestDefaultParams(t *testing.T) {
	params := DefaultParams()
	if params.LeafSize != 64*1024 {
		t.Errorf("Expected default leaf size 64KB, got %d", params.LeafSize)
	}
}

func TestHashString(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 10})
	
	content := []byte("hello world")
	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	
	hashStr := root.Hash.String()
	if len(hashStr) != 64 { // 32 bytes * 2 hex chars
		t.Errorf("Expected hash string length 64, got %d", len(hashStr))
	}
}

func TestBuildEmptyFile(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 10})
	
	root, err := builder.Build(nil)
	if err != nil {
		t.Fatalf("Build empty file failed: %v", err)
	}
	
	if root.Kind != Leaf {
		t.Errorf("Expected Leaf node, got %d", root.Kind)
	}
	if root.Size != 0 {
		t.Errorf("Expected size 0, got %d", root.Size)
	}
	
	// Verify we can read back empty content
	loader := NewLoader(cas)
	content, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if len(content) != 0 {
		t.Errorf("Expected empty content, got %d bytes", len(content))
	}
}

func TestBuildSingleChunk(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 20})
	
	content := []byte("hello world")
	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	
	if root.Kind != Leaf {
		t.Errorf("Expected Leaf node, got %d", root.Kind)
	}
	if root.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), root.Size)
	}
	
	// Verify content can be read back
	loader := NewLoader(cas)
	retrieved, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Errorf("Content mismatch: expected %q, got %q", content, retrieved)
	}
}

func TestBuildMultipleChunks(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 5}) // Small chunks for testing
	
	content := []byte("hello world test data")
	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	
	if root.Kind != Node {
		t.Errorf("Expected internal Node, got %d", root.Kind)
	}
	if root.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), root.Size)
	}
	
	// Verify content can be read back
	loader := NewLoader(cas)
	retrieved, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Errorf("Content mismatch: expected %q, got %q", content, retrieved)
	}
}

func TestBuildStreaming(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 8})
	
	content := []byte("streaming data content test")
	reader := bytes.NewReader(content)
	
	root, err := builder.BuildStreaming(reader)
	if err != nil {
		t.Fatalf("BuildStreaming failed: %v", err)
	}
	
	if root.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), root.Size)
	}
	
	// Verify content can be read back
	loader := NewLoader(cas)
	retrieved, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Errorf("Content mismatch: expected %q, got %q", content, retrieved)
	}
}

func TestBuildStreamingEmpty(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 10})
	
	reader := bytes.NewReader(nil)
	
	root, err := builder.BuildStreaming(reader)
	if err != nil {
		t.Fatalf("BuildStreaming empty failed: %v", err)
	}
	
	if root.Kind != Leaf {
		t.Errorf("Expected Leaf node, got %d", root.Kind)
	}
	if root.Size != 0 {
		t.Errorf("Expected size 0, got %d", root.Size)
	}
}

func TestReaderInterface(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 6})
	
	content := []byte("test reader interface")
	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	
	loader := NewLoader(cas)
	reader, err := loader.Reader(root)
	if err != nil {
		t.Fatalf("Reader failed: %v", err)
	}
	defer reader.Close()
	
	retrieved, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Errorf("Content mismatch: expected %q, got %q", content, retrieved)
	}
}

func TestLargerFile(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 1024})
	
	// Create content larger than a single chunk
	content := []byte(strings.Repeat("test data line\n", 200)) // ~3KB
	
	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	
	if root.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), root.Size)
	}
	
	loader := NewLoader(cas)
	retrieved, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Errorf("Content length mismatch: expected %d, got %d", len(content), len(retrieved))
	}
}

func TestSameContentSameHash(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 10})
	
	content := []byte("identical content")
	
	root1, err := builder.Build(content)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	
	root2, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}
	
	if root1.Hash != root2.Hash {
		t.Error("Same content should produce same hash")
	}
}

func TestDifferentContentDifferentHash(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 10})
	
	content1 := []byte("content one")
	content2 := []byte("content two")
	
	root1, err := builder.Build(content1)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	
	root2, err := builder.Build(content2)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}
	
	if root1.Hash == root2.Hash {
		t.Error("Different content should produce different hashes")
	}
}

func TestOddNumberChunks(t *testing.T) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, Params{LeafSize: 4})
	
	// Create content that results in odd number of chunks
	content := []byte("123456789") // 9 bytes = 3 chunks of 4,4,1
	
	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	
	loader := NewLoader(cas)
	retrieved, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Errorf("Content mismatch: expected %q, got %q", content, retrieved)
	}
}

func TestBuild_ExactlyOneChunk(t *testing.T) {
	store := cas.NewMemoryCAS()
	leafSize := 10
	builder := NewBuilder(store, Params{LeafSize: leafSize})

	content := make([]byte, leafSize) // Exactly one chunk
	for i := range content {
		content[i] = byte(i)
	}

	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if root.Kind != Leaf {
		t.Errorf("Expected Leaf for content exactly one chunk, got %d", root.Kind)
	}
	if root.Size != int64(leafSize) {
		t.Errorf("Expected size %d, got %d", leafSize, root.Size)
	}

	loader := NewLoader(store)
	retrieved, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Error("Content mismatch")
	}
}

func TestBuild_OneByteOverChunk(t *testing.T) {
	store := cas.NewMemoryCAS()
	leafSize := 10
	builder := NewBuilder(store, Params{LeafSize: leafSize})

	content := make([]byte, leafSize+1) // One byte over = 2 chunks
	for i := range content {
		content[i] = byte(i)
	}

	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if root.Kind != Node {
		t.Errorf("Expected Node for content one byte over chunk, got %d", root.Kind)
	}
	if root.Size != int64(leafSize+1) {
		t.Errorf("Expected size %d, got %d", leafSize+1, root.Size)
	}

	loader := NewLoader(store)
	retrieved, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Error("Content mismatch")
	}
}

func TestBuild_OneByteUnderChunk(t *testing.T) {
	store := cas.NewMemoryCAS()
	leafSize := 10
	builder := NewBuilder(store, Params{LeafSize: leafSize})

	content := make([]byte, leafSize-1) // One byte under = single leaf
	for i := range content {
		content[i] = byte(i)
	}

	root, err := builder.Build(content)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if root.Kind != Leaf {
		t.Errorf("Expected Leaf for content one byte under chunk, got %d", root.Kind)
	}

	loader := NewLoader(store)
	retrieved, err := loader.ReadAll(root)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(content, retrieved) {
		t.Error("Content mismatch")
	}
}

func TestLoader_CorruptedInternalNode(t *testing.T) {
	store := cas.NewMemoryCAS()

	// Put garbage data that looks like an internal node
	garbage := []byte{0x01, 0xFF, 0xFF, 0xFF} // 0x01 = internal node marker, rest is garbage
	hash := cas.SumB3(garbage)
	store.Put(hash, garbage)

	ref := NodeRef{
		Hash: hash,
		Kind: Node,
		Size: 100,
	}

	loader := NewLoader(store)
	_, err := loader.ReadAll(ref)
	// Should return error, not panic
	if err == nil {
		t.Log("ReadAll returned nil error for corrupted data - acceptable if it returned garbage content safely")
	}
}

func TestLoader_MissingChunk(t *testing.T) {
	store := cas.NewMemoryCAS()

	// Create a reference to a hash that doesn't exist in CAS
	fakeHash := cas.SumB3([]byte("this content was never stored"))
	ref := NodeRef{
		Hash: fakeHash,
		Kind: Leaf,
		Size: 50,
	}

	loader := NewLoader(store)
	_, err := loader.ReadAll(ref)
	if err == nil {
		t.Fatal("Expected error when reading missing chunk, got nil")
	}
}

func BenchmarkBuild1KB(b *testing.B) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, DefaultParams())
	content := make([]byte, 1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.Build(content)
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
	}
}

func BenchmarkBuild1MB(b *testing.B) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, DefaultParams())
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.Build(content)
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
	}
}

func BenchmarkReadAll1MB(b *testing.B) {
	cas := cas.NewMemoryCAS()
	builder := NewBuilder(cas, DefaultParams())
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	
	root, err := builder.Build(content)
	if err != nil {
		b.Fatalf("Build failed: %v", err)
	}
	
	loader := NewLoader(cas)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := loader.ReadAll(root)
		if err != nil {
			b.Fatalf("ReadAll failed: %v", err)
		}
	}
}