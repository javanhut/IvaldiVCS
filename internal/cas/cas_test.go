package cas

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
)

func TestSumB3(t *testing.T) {
	data := []byte("hello world")
	hash1 := SumB3(data)
	hash2 := SumB3(data)

	if hash1 != hash2 {
		t.Error("Same data should produce same hash")
	}

	// Test different data produces different hash
	hash3 := SumB3([]byte("hello world!"))
	if hash1 == hash3 {
		t.Error("Different data should produce different hashes")
	}
}

func TestMemoryCAS(t *testing.T) {
	cas := NewMemoryCAS()
	data := []byte("test data")
	hash := SumB3(data)

	// Test Has on empty CAS
	has, err := cas.Has(hash)
	if err != nil {
		t.Fatalf("Has failed: %v", err)
	}
	if has {
		t.Error("Empty CAS should not have any data")
	}

	// Test Get on empty CAS
	_, err = cas.Get(hash)
	if err == nil {
		t.Error("Get should fail on missing hash")
	}

	// Test Put
	err = cas.Put(hash, data)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Test Has after Put
	has, err = cas.Has(hash)
	if err != nil {
		t.Fatalf("Has failed: %v", err)
	}
	if !has {
		t.Error("CAS should have data after Put")
	}

	// Test Get after Put
	retrieved, err := cas.Get(hash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(data, retrieved) {
		t.Error("Retrieved data should match original")
	}

	// Test Put with wrong hash
	wrongHash := SumB3([]byte("different data"))
	err = cas.Put(wrongHash, data)
	if err == nil {
		t.Error("Put should fail with mismatched hash")
	}
}

func TestMemoryCASConcurrency(t *testing.T) {
	cas := NewMemoryCAS()
	data := []byte("concurrent test data")
	hash := SumB3(data)

	// First ensure data is written
	err := cas.Put(hash, data)
	if err != nil {
		t.Fatalf("initial Put failed: %v", err)
	}

	// Test concurrent access
	done := make(chan error, 15)

	// Multiple goroutines writing the same data
	for i := 0; i < 5; i++ {
		go func() {
			err := cas.Put(hash, data)
			done <- err
		}()
	}

	// Multiple goroutines reading
	for i := 0; i < 5; i++ {
		go func() {
			retrieved, err := cas.Get(hash)
			if err != nil {
				done <- err
				return
			}
			if !bytes.Equal(data, retrieved) {
				done <- bytes.ErrTooLarge
				return
			}
			done <- nil
		}()
	}

	// Multiple goroutines checking Has
	for i := 0; i < 5; i++ {
		go func() {
			has, err := cas.Has(hash)
			if err != nil {
				done <- err
				return
			}
			if !has {
				done <- bytes.ErrTooLarge
				return
			}
			done <- nil
		}()
	}

	// Wait for all goroutines and check for errors
	errors := 0
	for i := 0; i < 15; i++ {
		if err := <-done; err != nil {
			errors++
			t.Logf("Goroutine %d failed: %v", i, err)
		}
	}

	if errors > 0 {
		t.Errorf("Concurrent operations failed: %d goroutines reported errors", errors)
	}
}

func BenchmarkSumB3(b *testing.B) {
	data := make([]byte, 1024) // 1KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SumB3(data)
	}
}

func BenchmarkMemoryCASPut(b *testing.B) {
	cas := NewMemoryCAS()
	data := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash := SumB3(append(data, byte(i%256)))
		cas.Put(hash, data)
	}
}

func BenchmarkMemoryCASGet(b *testing.B) {
	cas := NewMemoryCAS()
	data := []byte("benchmark data")
	hash := SumB3(data)
	cas.Put(hash, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cas.Get(hash)
	}
}

// --- FileCAS Tests ---

func newTestFileCAS(t *testing.T) (*FileCAS, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "filecas-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	fc, err := NewFileCAS(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("NewFileCAS failed: %v", err)
	}
	return fc, dir
}

func TestFileCAS_PutAndGet(t *testing.T) {
	fc, dir := newTestFileCAS(t)
	defer os.RemoveAll(dir)

	data := []byte("hello file cas")
	hash := SumB3(data)

	if err := fc.Put(hash, data); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	retrieved, err := fc.Get(hash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(data, retrieved) {
		t.Error("Retrieved data does not match original")
	}
}

func TestFileCAS_Has(t *testing.T) {
	fc, dir := newTestFileCAS(t)
	defer os.RemoveAll(dir)

	data := []byte("existence check")
	hash := SumB3(data)

	// Should not exist yet
	has, err := fc.Has(hash)
	if err != nil {
		t.Fatalf("Has failed: %v", err)
	}
	if has {
		t.Error("Hash should not exist before Put")
	}

	if err := fc.Put(hash, data); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Should exist now
	has, err = fc.Has(hash)
	if err != nil {
		t.Fatalf("Has failed: %v", err)
	}
	if !has {
		t.Error("Hash should exist after Put")
	}
}

func TestFileCAS_GetMissing(t *testing.T) {
	fc, dir := newTestFileCAS(t)
	defer os.RemoveAll(dir)

	missingHash := SumB3([]byte("this data was never stored"))

	_, err := fc.Get(missingHash)
	if err == nil {
		t.Error("Get on missing hash should return an error")
	}
}

func TestFileCAS_PutWrongHash(t *testing.T) {
	fc, dir := newTestFileCAS(t)
	defer os.RemoveAll(dir)

	data := []byte("real data")
	wrongHash := SumB3([]byte("different data"))

	err := fc.Put(wrongHash, data)
	if err == nil {
		t.Error("Put with mismatched hash should return an error")
	}
}

func TestFileCAS_LargeData(t *testing.T) {
	fc, dir := newTestFileCAS(t)
	defer os.RemoveAll(dir)

	data := make([]byte, 1<<20) // 1 MB
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("failed to generate random data: %v", err)
	}

	hash := SumB3(data)

	if err := fc.Put(hash, data); err != nil {
		t.Fatalf("Put failed for large data: %v", err)
	}

	retrieved, err := fc.Get(hash)
	if err != nil {
		t.Fatalf("Get failed for large data: %v", err)
	}
	if !bytes.Equal(data, retrieved) {
		t.Error("Retrieved large data does not match original")
	}
}

func TestFileCAS_MultipleObjects(t *testing.T) {
	fc, dir := newTestFileCAS(t)
	defer os.RemoveAll(dir)

	objects := make(map[Hash][]byte)
	for i := 0; i < 20; i++ {
		data := []byte(fmt.Sprintf("object number %d", i))
		hash := SumB3(data)
		objects[hash] = data

		if err := fc.Put(hash, data); err != nil {
			t.Fatalf("Put failed for object %d: %v", i, err)
		}
	}

	for hash, expected := range objects {
		retrieved, err := fc.Get(hash)
		if err != nil {
			t.Fatalf("Get failed for hash %s: %v", hash, err)
		}
		if !bytes.Equal(expected, retrieved) {
			t.Errorf("Data mismatch for hash %s", hash)
		}

		has, err := fc.Has(hash)
		if err != nil {
			t.Fatalf("Has failed for hash %s: %v", hash, err)
		}
		if !has {
			t.Errorf("Has returned false for stored hash %s", hash)
		}
	}
}

func TestFileCAS_Persistence(t *testing.T) {
	dir, err := os.MkdirTemp("", "filecas-persist-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	data := []byte("persistent data")
	hash := SumB3(data)

	// Write with first instance
	fc1, err := NewFileCAS(dir)
	if err != nil {
		t.Fatalf("NewFileCAS (first) failed: %v", err)
	}
	if err := fc1.Put(hash, data); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Create a new instance on the same directory and read back
	fc2, err := NewFileCAS(dir)
	if err != nil {
		t.Fatalf("NewFileCAS (second) failed: %v", err)
	}

	has, err := fc2.Has(hash)
	if err != nil {
		t.Fatalf("Has failed on second instance: %v", err)
	}
	if !has {
		t.Error("Second FileCAS instance should find data written by first")
	}

	retrieved, err := fc2.Get(hash)
	if err != nil {
		t.Fatalf("Get failed on second instance: %v", err)
	}
	if !bytes.Equal(data, retrieved) {
		t.Error("Data from second instance does not match original")
	}
}

func TestFileCAS_InvalidDir(t *testing.T) {
	// Use a path under /dev/null which cannot be a directory
	_, err := NewFileCAS("/dev/null/impossible")
	if err == nil {
		t.Error("NewFileCAS should fail for an invalid directory path")
	}
}

func TestFileCAS_PutIdempotent(t *testing.T) {
	fc, dir := newTestFileCAS(t)
	defer os.RemoveAll(dir)

	data := []byte("idempotent data")
	hash := SumB3(data)

	// Put twice should succeed both times
	if err := fc.Put(hash, data); err != nil {
		t.Fatalf("First Put failed: %v", err)
	}
	if err := fc.Put(hash, data); err != nil {
		t.Fatalf("Second Put (idempotent) failed: %v", err)
	}

	retrieved, err := fc.Get(hash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(data, retrieved) {
		t.Error("Retrieved data does not match after idempotent Put")
	}
}

// --- BLAKE3 and Hash type tests ---

func TestHash_String(t *testing.T) {
	h := SumB3([]byte("test"))
	s := h.String()

	// Should be 64 hex chars
	if len(s) != 64 {
		t.Errorf("Hash.String() should be 64 chars, got %d", len(s))
	}

	// Should be all hex chars
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Hash.String() contains non-hex char: %c", c)
		}
	}
}

func TestHash_ZeroValue(t *testing.T) {
	var h Hash
	s := h.String()
	if s != "0000000000000000000000000000000000000000000000000000000000000000" {
		t.Errorf("Zero hash string unexpected: %s", s)
	}
}

func TestSumB3_EmptyData(t *testing.T) {
	h1 := SumB3([]byte{})
	h2 := SumB3([]byte{})
	if h1 != h2 {
		t.Error("Empty data should produce consistent hash")
	}
	if h1 == (Hash{}) {
		t.Error("BLAKE3 of empty data should not be zero hash")
	}
}

func TestSumB3_NilData(t *testing.T) {
	h := SumB3(nil)
	hEmpty := SumB3([]byte{})
	if h != hEmpty {
		t.Error("nil and empty byte slice should produce same hash")
	}
}

func TestMemoryCAS_Len(t *testing.T) {
	cas := NewMemoryCAS()
	if cas.Len() != 0 {
		t.Errorf("Empty CAS should have Len 0, got %d", cas.Len())
	}

	data1 := []byte("data one")
	cas.Put(SumB3(data1), data1)
	if cas.Len() != 1 {
		t.Errorf("Expected Len 1, got %d", cas.Len())
	}

	data2 := []byte("data two")
	cas.Put(SumB3(data2), data2)
	if cas.Len() != 2 {
		t.Errorf("Expected Len 2, got %d", cas.Len())
	}

	// Putting same data again shouldn't increase count
	cas.Put(SumB3(data1), data1)
	if cas.Len() != 2 {
		t.Errorf("Duplicate put should not increase Len, got %d", cas.Len())
	}
}

func TestMemoryCAS_DataIsolation(t *testing.T) {
	cas := NewMemoryCAS()
	original := []byte("immutable data")
	hash := SumB3(original)
	cas.Put(hash, original)

	// Mutate original after Put
	original[0] = 'X'

	// CAS should still return unmutated copy
	retrieved, _ := cas.Get(hash)
	if retrieved[0] == 'X' {
		t.Error("CAS should store a copy, not reference original")
	}

	// Mutate retrieved value
	retrieved[0] = 'Y'

	// CAS should still return unmutated copy
	retrieved2, _ := cas.Get(hash)
	if retrieved2[0] == 'Y' {
		t.Error("CAS should return a copy on Get, not internal reference")
	}
}
