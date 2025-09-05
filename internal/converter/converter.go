package converter

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/javanhut/Ivaldi-vcs/internal/keys"
	"github.com/javanhut/Ivaldi-vcs/internal/objects"
	"github.com/javanhut/Ivaldi-vcs/internal/pack"
	"github.com/javanhut/Ivaldi-vcs/internal/store"
)

// ConversionResult holds the results of converting objects.
type ConversionResult struct {
	Converted int
	Skipped   int
	Errors    []error
}

// ConvertGitObjectsToIvaldi discovers all Git objects and converts them to Ivaldi format.
func ConvertGitObjectsToIvaldi(gitDir, ivaldiDir string) (*ConversionResult, error) {
	result := &ConversionResult{}

	// Open the Ivaldi KV store
	fmt.Println("Opening shared Ivaldi KV store for Git conversion...")
	db, err := store.GetSharedDB(ivaldiDir)
	if err != nil {
		return result, fmt.Errorf("open shared ivaldi store: %w", err)
	}
	defer db.Close()
	fmt.Println("Shared KV store opened successfully")

	// Create objects directory
	objectsDir := filepath.Join(ivaldiDir, "objects")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		return result, fmt.Errorf("create objects dir: %w", err)
	}

	// Discover Git objects
	objectPaths, err := objects.DiscoverGitObjects(gitDir)
	if err != nil {
		return result, fmt.Errorf("discover git objects: %w", err)
	}

	// Convert each Git object
	for i, objectPath := range objectPaths {
		fmt.Printf("Converting Git object %d/%d: %s\n", i+1, len(objectPaths), objectPath)
		if err := convertSingleGitObject(objectPath, objectsDir, db); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("convert %s: %w", objectPath, err))
			result.Skipped++
		} else {
			result.Converted++
		}
	}

	return result, nil
}

// convertSingleGitObject converts one Git object to Ivaldi format.
func convertSingleGitObject(objectPath, ivaldiObjectsDir string, db *store.SharedDB) error {
	// Convert Git object to Ivaldi format
	digest, content, gitSHA1, err := objects.ConvertGitBlobToIvaldi(objectPath)
	if err != nil {
		return err
	}

	// Generate unique human-readable key
	humanKey, err := keys.GenerateUniquePhrase(db, 3, 4) // 3 words + 4 digits
	if err != nil {
		return fmt.Errorf("generate unique key: %w", err)
	}

	// Store KV mapping (human key -> blake3/sha256)
	if err := db.PutMapping(humanKey, digest.BLAKE3, digest.SHA256); err != nil {
		return fmt.Errorf("store kv mapping: %w", err)
	}

	// Store Git hash mapping (git sha1 -> blake3/sha256) if we have a git hash
	if gitSHA1 != "" {
		if err := db.PutGitMapping(gitSHA1, digest.BLAKE3, digest.SHA256); err != nil {
			return fmt.Errorf("store git mapping: %w", err)
		}
	}

	// Store compressed object in .ivaldi/objects
	compressed, err := objects.EncodeZstdGitBlob(content)
	if err != nil {
		return fmt.Errorf("compress object: %w", err)
	}

	// Use BLAKE3 hash as filename (first 2 chars as subdir)
	blake3Hex := hex.EncodeToString(digest.BLAKE3[:])
	subDir := filepath.Join(ivaldiObjectsDir, blake3Hex[:2])
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return fmt.Errorf("create object subdir: %w", err)
	}

	objectFile := filepath.Join(subDir, blake3Hex[2:])
	if err := os.WriteFile(objectFile, compressed, 0644); err != nil {
		return fmt.Errorf("write object file: %w", err)
	}

	return nil
}

// SnapshotCurrentFiles creates blob objects for all files in the working directory.
func SnapshotCurrentFiles(workDir, ivaldiDir string) (*ConversionResult, error) {
	result := &ConversionResult{}

	// Open the Ivaldi KV store
	db, err := store.GetSharedDB(ivaldiDir)
	if err != nil {
		return result, fmt.Errorf("open shared ivaldi store: %w", err)
	}
	defer db.Close()

	// Create objects directory
	objectsDir := filepath.Join(ivaldiDir, "objects")
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		return result, fmt.Errorf("create objects dir: %w", err)
	}

	// Walk working directory
	err = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files/dirs
		if info.IsDir() || filepath.Base(path)[0] == '.' {
			return nil
		}

		// Skip files in .git and .ivaldi directories
		relPath, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(relPath, ".git") || strings.HasPrefix(relPath, ".ivaldi") {
			return nil
		}

		// Create blob from file
		fmt.Printf("Snapshotting file: %s\n", relPath)
		if err := createBlobFromFile(path, relPath, objectsDir, db); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("snapshot %s: %w", relPath, err))
			result.Skipped++
		} else {
			result.Converted++
		}

		return nil
	})

	if err != nil {
		return result, fmt.Errorf("walk working directory: %w", err)
	}

	return result, nil
}

// createBlobFromFile creates an Ivaldi blob object from a file.
func createBlobFromFile(filePath, relPath, ivaldiObjectsDir string, db *store.SharedDB) error {
	// Read file content
	content, err := objects.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Generate dual hashes
	digest := &objects.DualDigest{
		SHA256: objects.HashBlobSHA256(content),
		BLAKE3: objects.HashBlobBLAKE3(content),
		Size:   len(content),
	}

	// Generate unique human-readable key based on file path
	baseKey := filepath.Base(relPath)
	humanKey, err := keys.GenerateUniquePhrase(db, 2, 3) // 2 words + 3 digits
	if err != nil {
		return fmt.Errorf("generate unique key: %w", err)
	}
	// Combine with file name for better readability
	humanKey = fmt.Sprintf("%s-%s", baseKey, humanKey)

	// Store KV mapping
	if err := db.PutMapping(humanKey, digest.BLAKE3, digest.SHA256); err != nil {
		return fmt.Errorf("store kv mapping: %w", err)
	}

	// Store compressed object
	compressed, err := objects.EncodeZstdGitBlob(content)
	if err != nil {
		return fmt.Errorf("compress object: %w", err)
	}

	// Use BLAKE3 hash as filename
	blake3Hex := hex.EncodeToString(digest.BLAKE3[:])
	subDir := filepath.Join(ivaldiObjectsDir, blake3Hex[:2])
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return fmt.Errorf("create object subdir: %w", err)
	}

	objectFile := filepath.Join(subDir, blake3Hex[2:])
	if err := os.WriteFile(objectFile, compressed, 0644); err != nil {
		return fmt.Errorf("write object file: %w", err)
	}

	return nil
}

// GenerateGitCompatiblePack converts Ivaldi objects back to Git-compatible pack format.
func GenerateGitCompatiblePack(ivaldiDir string, humanKeys []string) ([]byte, error) {
	// Open the Ivaldi KV store
	db, err := store.GetSharedDB(ivaldiDir)
	if err != nil {
		return nil, fmt.Errorf("open shared ivaldi store: %w", err)
	}
	defer db.Close()

	objectsDir := filepath.Join(ivaldiDir, "objects")
	var packObjects []pack.Object

	// Convert each requested object
	for _, humanKey := range humanKeys {
		blake3Hex, sha256Hex, err := db.LookupByKey(humanKey)
		if err != nil {
			return nil, fmt.Errorf("lookup key %s: %w", humanKey, err)
		}

		// Read the compressed Ivaldi object
		objectPath := filepath.Join(objectsDir, blake3Hex[:2], blake3Hex[2:])
		objectFile, err := os.Open(objectPath)
		if err != nil {
			return nil, fmt.Errorf("open object file %s: %w", objectPath, err)
		}

		// Decompress and extract content
		blob, err := objects.DecodeZstdGitBlob(objectFile)
		objectFile.Close()
		if err != nil {
			return nil, fmt.Errorf("decode ivaldi object %s: %w", humanKey, err)
		}

		// Create canonical Git blob bytes
		canonicalBytes := gitHeader("blob", len(blob.Content))
		canonicalBytes = append(canonicalBytes, blob.Content...)

		// Add to pack objects
		packObjects = append(packObjects, pack.Object{
			Type: 3, // objBlob
			Size: uint64(len(canonicalBytes)),
			Data: canonicalBytes,
			Algo: pack.CompressZlib, // Use zlib for Git compatibility
		})

		_ = sha256Hex // We have the SHA256 if needed for verification
	}

	// Generate the pack file using concurrent compression (8 workers)
	packData, err := pack.WritePackConcurrent(packObjects, false, 8) // No SHA256 trailer for Git compatibility
	if err != nil {
		return nil, fmt.Errorf("write pack file: %w", err)
	}

	return packData, nil
}

// gitHeader creates a Git object header.
func gitHeader(objType string, size int) []byte {
	return []byte(fmt.Sprintf("%s %d\x00", objType, size))
}
