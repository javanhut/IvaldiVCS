package cas

import (
	"bytes"
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

	// Test concurrent access
	done := make(chan bool, 10)
	
	// Multiple goroutines writing the same data
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			err := cas.Put(hash, data)
			if err != nil {
				t.Errorf("Concurrent Put failed: %v", err)
			}
		}()
	}

	// Multiple goroutines reading
	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()
			// Wait a bit for puts to complete
			for j := 0; j < 100; j++ {
				retrieved, err := cas.Get(hash)
				if err == nil && bytes.Equal(data, retrieved) {
					return
				}
			}
			t.Error("Concurrent Get failed")
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
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