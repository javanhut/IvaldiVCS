// Package cas provides a content-addressable storage interface and BLAKE3 hashing utilities.
package cas

import (
	"encoding/hex"
	"fmt"
	"sync"

	"lukechampine.com/blake3"
)

// Hash represents a BLAKE3-256 hash value.
type Hash [32]byte

// String returns the hexadecimal representation of the hash.
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// SumB3 computes the BLAKE3 hash of the given data.
func SumB3(data []byte) Hash {
	return blake3.Sum256(data)
}

// CAS defines the content-addressable storage interface.
type CAS interface {
	// Put stores data keyed by its hash.
	Put(hash Hash, data []byte) error

	// Get retrieves data by its hash.
	Get(hash Hash) ([]byte, error)

	// Has checks if data exists for the given hash.
	Has(hash Hash) (bool, error)
}

// MemoryCAS implements CAS using in-memory storage with thread-safe access.
type MemoryCAS struct {
	mu   sync.RWMutex
	data map[Hash][]byte
}

// NewMemoryCAS creates a new in-memory CAS.
func NewMemoryCAS() *MemoryCAS {
	return &MemoryCAS{
		data: make(map[Hash][]byte),
	}
}

// Put implements CAS.Put.
func (m *MemoryCAS) Put(hash Hash, data []byte) error {
	// Verify hash matches content
	computed := SumB3(data)
	if computed != hash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", hash, computed)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Store a copy to avoid external mutations
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	m.data[hash] = dataCopy

	return nil
}

// Get implements CAS.Get.
func (m *MemoryCAS) Get(hash Hash) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.data[hash]
	if !exists {
		return nil, fmt.Errorf("hash not found: %s", hash)
	}

	// Return a copy to avoid external mutations
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

// Has implements CAS.Has.
func (m *MemoryCAS) Has(hash Hash) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.data[hash]
	return exists, nil
}

// Len returns the number of objects stored in the CAS.
func (m *MemoryCAS) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}