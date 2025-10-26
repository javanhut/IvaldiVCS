package butterfly

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/javanhut/Ivaldi-vcs/internal/cas"
	bolt "go.etcd.io/bbolt"
)

var (
	butterflyBucket = []byte("butterflies")
	parentBucket    = []byte("parents")
	timelineBucket  = []byte("timelines")
)

type MetadataStore struct {
	db *bolt.DB
}

func NewMetadataStore(ivaldiDir string) (*MetadataStore, error) {
	dbPath := filepath.Join(ivaldiDir, "butterflies", "metadata.db")
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open butterfly metadata db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(butterflyBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(parentBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(timelineBucket); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	return &MetadataStore{db: db}, nil
}

func (s *MetadataStore) Close() error {
	return s.db.Close()
}

func (s *MetadataStore) StoreButterfly(bf *Butterfly) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(butterflyBucket)
		data, err := json.Marshal(bf)
		if err != nil {
			return fmt.Errorf("failed to marshal butterfly: %w", err)
		}
		return b.Put([]byte(bf.Name), data)
	})
}

func (s *MetadataStore) GetButterfly(name string) (*Butterfly, error) {
	var bf Butterfly
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(butterflyBucket)
		data := b.Get([]byte(name))
		if data == nil {
			return fmt.Errorf("butterfly not found: %s", name)
		}
		return json.Unmarshal(data, &bf)
	})
	if err != nil {
		return nil, err
	}
	return &bf, nil
}

func (s *MetadataStore) DeleteButterfly(name string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(butterflyBucket)
		return b.Delete([]byte(name))
	})
}

func (s *MetadataStore) AddChild(parentName, childName string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(parentBucket)
		data := b.Get([]byte(parentName))
		var children []string
		if data != nil {
			if err := json.Unmarshal(data, &children); err != nil {
				return err
			}
		}
		children = append(children, childName)
		newData, err := json.Marshal(children)
		if err != nil {
			return err
		}
		return b.Put([]byte(parentName), newData)
	})
}

func (s *MetadataStore) RemoveChild(parentName, childName string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(parentBucket)
		data := b.Get([]byte(parentName))
		if data == nil {
			return nil
		}
		var children []string
		if err := json.Unmarshal(data, &children); err != nil {
			return err
		}
		filtered := []string{}
		for _, child := range children {
			if child != childName {
				filtered = append(filtered, child)
			}
		}
		if len(filtered) == 0 {
			return b.Delete([]byte(parentName))
		}
		newData, err := json.Marshal(filtered)
		if err != nil {
			return err
		}
		return b.Put([]byte(parentName), newData)
	})
}

func (s *MetadataStore) GetChildren(parentName string) ([]string, error) {
	var children []string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(parentBucket)
		data := b.Get([]byte(parentName))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &children)
	})
	return children, err
}

func (s *MetadataStore) GetMetadata(timelineName string) (*ButterflyMetadata, error) {
	bf, err := s.GetButterfly(timelineName)
	if err != nil {
		return &ButterflyMetadata{
			Timeline:    timelineName,
			IsButterfly: false,
			Butterfly:   nil,
			Children:    []string{},
		}, nil
	}

	children, _ := s.GetChildren(timelineName)

	return &ButterflyMetadata{
		Timeline:    timelineName,
		IsButterfly: true,
		Butterfly:   bf,
		Children:    children,
	}, nil
}

func (s *MetadataStore) ListAllButterflies() ([]*Butterfly, error) {
	var butterflies []*Butterfly
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(butterflyBucket)
		return b.ForEach(func(k, v []byte) error {
			var bf Butterfly
			if err := json.Unmarshal(v, &bf); err != nil {
				return err
			}
			butterflies = append(butterflies, &bf)
			return nil
		})
	})
	return butterflies, err
}

func (s *MetadataStore) MarkOrphaned(name string, originalParent string) error {
	bf, err := s.GetButterfly(name)
	if err != nil {
		return err
	}
	bf.IsOrphaned = true
	bf.OriginalParent = originalParent
	return s.StoreButterfly(bf)
}

func hashToBytes(h cas.Hash) []byte {
	return h[:]
}

func bytesToHash(b []byte) cas.Hash {
	var h cas.Hash
	copy(h[:], b)
	return h
}
