package store

import (
	"encoding/hex"
	"errors"

	"go.etcd.io/bbolt"
)

// Buckets
var (
	BucketKeyToB3   = []byte("key->b3")   // human key -> blake3 hex
	BucketB3ToS2    = []byte("b3->s256")  // blake3 hex -> sha256 hex
	BucketGitToB3   = []byte("git->b3")   // git sha1 hex -> blake3 hex
	BucketGitToS256 = []byte("git->s256") // git sha1 hex -> sha256 hex
	BucketConfig    = []byte("config")    // repository configuration
)

type DB struct{ *bbolt.DB }

func Open(path string) (*DB, error) {
	db, err := bbolt.Open(path, 0666, nil)
	if err != nil {
		return nil, err
	}
	// Ensure buckets exist
	if err := db.Update(func(tx *bbolt.Tx) error {
		if _, e := tx.CreateBucketIfNotExists(BucketKeyToB3); e != nil {
			return e
		}
		if _, e := tx.CreateBucketIfNotExists(BucketB3ToS2); e != nil {
			return e
		}
		if _, e := tx.CreateBucketIfNotExists(BucketGitToB3); e != nil {
			return e
		}
		if _, e := tx.CreateBucketIfNotExists(BucketGitToS256); e != nil {
			return e
		}
		if _, e := tx.CreateBucketIfNotExists(BucketConfig); e != nil {
			return e
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db}, nil
}

func (db *DB) Close() error { return db.DB.Close() }

// PutMapping stores key -> blake3, and blake3 -> sha256 (all hex strings).
func (db *DB) PutMapping(humanKey string, blake3_32 [32]byte, sha256_32 [32]byte) error {
	b3hex := hex.EncodeToString(blake3_32[:])
	s2hex := hex.EncodeToString(sha256_32[:])

	return db.Update(func(tx *bbolt.Tx) error {
		if err := tx.Bucket(BucketKeyToB3).Put([]byte(humanKey), []byte(b3hex)); err != nil {
			return err
		}
		if err := tx.Bucket(BucketB3ToS2).Put([]byte(b3hex), []byte(s2hex)); err != nil {
			return err
		}
		return nil
	})
}

func (db *DB) LookupByKey(humanKey string) (blake3Hex, sha256Hex string, err error) {
	err = db.View(func(tx *bbolt.Tx) error {
		b3 := tx.Bucket(BucketKeyToB3).Get([]byte(humanKey))
		if b3 == nil {
			return errors.New("key not found")
		}
		s2 := tx.Bucket(BucketB3ToS2).Get(b3)
		if s2 == nil {
			return errors.New("no sha256 mapping for blake3")
		}
		blake3Hex = string(b3)
		sha256Hex = string(s2)
		return nil
	})
	return
}

// PutGitMapping stores git sha1 -> blake3 and git sha1 -> sha256 mappings.
func (db *DB) PutGitMapping(gitSHA1 string, blake3_32 [32]byte, sha256_32 [32]byte) error {
	b3hex := hex.EncodeToString(blake3_32[:])
	s2hex := hex.EncodeToString(sha256_32[:])

	return db.Update(func(tx *bbolt.Tx) error {
		if err := tx.Bucket(BucketGitToB3).Put([]byte(gitSHA1), []byte(b3hex)); err != nil {
			return err
		}
		if err := tx.Bucket(BucketGitToS256).Put([]byte(gitSHA1), []byte(s2hex)); err != nil {
			return err
		}
		return nil
	})
}

// LookupByGitHash looks up blake3 and sha256 hashes by git sha1 hash.
func (db *DB) LookupByGitHash(gitSHA1 string) (blake3Hex, sha256Hex string, err error) {
	err = db.View(func(tx *bbolt.Tx) error {
		b3 := tx.Bucket(BucketGitToB3).Get([]byte(gitSHA1))
		if b3 == nil {
			return errors.New("git hash not found")
		}
		s2 := tx.Bucket(BucketGitToS256).Get([]byte(gitSHA1))
		if s2 == nil {
			return errors.New("no sha256 mapping for git hash")
		}
		blake3Hex = string(b3)
		sha256Hex = string(s2)
		return nil
	})
	return
}

// GetAllGitHashes returns all stored git sha1 hashes.
func (db *DB) GetAllGitHashes() ([]string, error) {
	var hashes []string
	err := db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(BucketGitToB3).ForEach(func(k, v []byte) error {
			hashes = append(hashes, string(k))
			return nil
		})
	})
	return hashes, err
}

// PutConfig stores a configuration key-value pair.
func (db *DB) PutConfig(key, value string) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(BucketConfig).Put([]byte(key), []byte(value))
	})
}

// GetConfig retrieves a configuration value by key.
func (db *DB) GetConfig(key string) (string, error) {
	var value string
	err := db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(BucketConfig).Get([]byte(key))
		if v == nil {
			return errors.New("config key not found")
		}
		value = string(v)
		return nil
	})
	return value, err
}

// RemoveConfig removes a configuration key-value pair.
func (db *DB) RemoveConfig(key string) error {
	return db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket(BucketConfig).Delete([]byte(key))
	})
}
