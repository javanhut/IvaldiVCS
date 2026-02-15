package history

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	"github.com/javanhut/Ivaldi-vcs/internal/store"
	"go.etcd.io/bbolt"
)

// PersistentMMR implements an MMR backed by persistent storage.
type PersistentMMR struct {
	*MMR
	cas cas.CAS
	db  *store.SharedDB
}

// NewPersistentMMR creates a new MMR with persistent storage.
func NewPersistentMMR(casStore cas.CAS, ivaldiDir string) (*PersistentMMR, error) {
	// Get shared database connection
	db, err := store.GetSharedDB(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create base MMR
	mmr := NewMMR()

	p := &PersistentMMR{
		MMR: mmr,
		cas: casStore,
		db:  db,
	}

	// Load existing MMR state from storage
	if err := p.loadFromStorage(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load MMR state: %w", err)
	}

	return p, nil
}

// AppendLeaf appends a leaf and persists the state in a single transaction.
func (p *PersistentMMR) AppendLeaf(l Leaf) (uint64, Hash, error) {
	// Call parent implementation
	idx, root, err := p.MMR.AppendLeaf(l)
	if err != nil {
		return 0, Hash{}, err
	}

	// Persist leaf + MMR state in a single transaction
	if err := p.persistAll(idx, l); err != nil {
		return 0, Hash{}, fmt.Errorf("failed to persist leaf and state: %w", err)
	}

	return idx, root, nil
}

// loadFromStorage loads the MMR state from persistent storage.
func (p *PersistentMMR) loadFromStorage() error {
	// Load MMR metadata (size, peaks, etc.)
	var metaData []byte
	err := p.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("mmr"))
		if bucket == nil {
			return nil
		}
		metaData = bucket.Get([]byte("metadata"))
		return nil
	})
	if err != nil {
		return nil // No existing state, start fresh
	}
	if metaData == nil {
		return nil // No existing state
	}

	var metadata struct {
		Size  uint64   `json:"size"`
		Peaks []uint64 `json:"peaks"`
	}
	if err := json.Unmarshal(metaData, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Load all leaves and nodes in a single transaction
	err = p.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("mmr"))
		if bucket == nil {
			return nil
		}

		// Load leaves
		for i := uint64(0); i < metadata.Size; i++ {
			leafKey := p.leafKey(i)
			leafData := bucket.Get(leafKey)
			if leafData == nil {
				return fmt.Errorf("missing leaf %d", i)
			}

			var leaf Leaf
			if err := json.Unmarshal(leafData, &leaf); err != nil {
				return fmt.Errorf("failed to unmarshal leaf %d: %w", i, err)
			}

			p.leaves = append(p.leaves, leaf)
		}

		// Load all node trees from peaks within the same transaction
		for _, peakPos := range metadata.Peaks {
			p.loadNodeTreeFromBucket(bucket, peakPos)
		}

		return nil
	})
	if err != nil {
		return err
	}

	p.peaks = metadata.Peaks

	return nil
}

// loadNodeTreeFromBucket recursively loads nodes from a bucket within an existing transaction.
func (p *PersistentMMR) loadNodeTreeFromBucket(bucket *bbolt.Bucket, pos uint64) {
	nodeKey := p.nodeKey(pos)
	nodeData := bucket.Get(nodeKey)
	if nodeData == nil {
		return // Node doesn't exist (might be a leaf)
	}

	if len(nodeData) != 32 {
		return
	}

	var hash Hash
	copy(hash[:], nodeData)
	p.nodes[pos] = hash

	// Recursively load children if this is an internal node
	height := p.getHeight(pos)
	if height > 0 {
		step := uint64(1) << (height - 1)
		leftPos := pos - step
		rightPos := pos - 1

		p.loadNodeTreeFromBucket(bucket, leftPos)
		p.loadNodeTreeFromBucket(bucket, rightPos)
	}
}

// persistAll persists a leaf, metadata, and all nodes in a single transaction.
func (p *PersistentMMR) persistAll(idx uint64, leaf Leaf) error {
	leafData, err := json.Marshal(leaf)
	if err != nil {
		return fmt.Errorf("failed to marshal leaf: %w", err)
	}

	metadata := struct {
		Size  uint64   `json:"size"`
		Peaks []uint64 `json:"peaks"`
	}{
		Size:  uint64(len(p.leaves)),
		Peaks: p.peaks,
	}

	metaData, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return p.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("mmr"))
		if err != nil {
			return err
		}

		// Save leaf
		leafKey := p.leafKey(idx)
		if err := bucket.Put(leafKey, leafData); err != nil {
			return fmt.Errorf("failed to save leaf: %w", err)
		}

		// Save metadata
		if err := bucket.Put([]byte("metadata"), metaData); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		// Save all nodes
		for pos, hash := range p.nodes {
			nodeKey := p.nodeKey(pos)
			if err := bucket.Put(nodeKey, hash[:]); err != nil {
				return fmt.Errorf("failed to save node %d: %w", pos, err)
			}
		}

		return nil
	})
}

// leafKey generates a storage key for a leaf.
func (p *PersistentMMR) leafKey(idx uint64) []byte {
	key := make([]byte, 12)
	copy(key[:4], []byte("mmr:"))
	copy(key[4:8], []byte("leaf"))
	binary.BigEndian.PutUint32(key[8:], uint32(idx))
	return key
}

// nodeKey generates a storage key for an internal node.
func (p *PersistentMMR) nodeKey(pos uint64) []byte {
	key := make([]byte, 12)
	copy(key[:4], []byte("mmr:"))
	copy(key[4:8], []byte("node"))
	binary.BigEndian.PutUint32(key[8:], uint32(pos))
	return key
}

// Close closes the persistent MMR.
func (p *PersistentMMR) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// PersistentTimelineStore implements TimelineStore with persistent storage.
type PersistentTimelineStore struct {
	db *store.SharedDB
}

// NewPersistentTimelineStore creates a new persistent timeline store.
func NewPersistentTimelineStore(ivaldiDir string) (*PersistentTimelineStore, error) {
	db, err := store.GetSharedDB(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	return &PersistentTimelineStore{db: db}, nil
}

// GetHead returns the head leaf index for a timeline.
func (s *PersistentTimelineStore) GetHead(name string) (uint64, bool) {
	var idx uint64
	found := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("timelines"))
		if bucket == nil {
			return nil
		}

		key := s.timelineKey(name)
		data := bucket.Get(key)
		if data == nil {
			return nil
		}

		if len(data) != 8 {
			return fmt.Errorf("invalid data size")
		}

		idx = binary.BigEndian.Uint64(data)
		found = true
		return nil
	})

	if err != nil {
		return 0, false
	}
	return idx, found
}

// SetHead sets the head leaf index for a timeline.
func (s *PersistentTimelineStore) SetHead(name string, idx uint64) error {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, idx)

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("timelines"))
		if err != nil {
			return err
		}
		key := s.timelineKey(name)
		return bucket.Put(key, data)
	})
}

// List returns all timeline names.
func (s *PersistentTimelineStore) List() []string {
	// This would require iterating over keys with prefix "timeline:"
	// For now, return empty list
	return []string{}
}

// timelineKey generates a storage key for a timeline head.
func (s *PersistentTimelineStore) timelineKey(name string) []byte {
	return []byte(fmt.Sprintf("timeline:%s", name))
}

// Close closes the timeline store.
func (s *PersistentTimelineStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
