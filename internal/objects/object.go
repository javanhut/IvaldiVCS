package objects

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"lukechampine.com/blake3"
)

// Blob holds raw file content (Git "blob" object payload).
type Blob struct {
	Size    int
	Content []byte
}

// ---------------------------
// Canonical Git blob encoding
// ---------------------------

func gitHeader(objType string, size int) []byte {
	return []byte(fmt.Sprintf("%s %d\x00", objType, size))
}

// Canonical bytes Git hashes: "blob <len>\x00" + content
func canonicalBlobBytes(content []byte) []byte {
	h := gitHeader("blob", len(content))
	out := make([]byte, 0, len(h)+len(content))
	out = append(out, h...)
	out = append(out, content...)
	return out
}

// ---------------------------
// Hashing (SHA-256 & BLAKE3)
// ---------------------------

func HashBlobSHA256(content []byte) [32]byte {
	return sha256.Sum256(canonicalBlobBytes(content))
}

func HashBlobBLAKE3(content []byte) [32]byte {
	return blake3.Sum256(canonicalBlobBytes(content))
}

// ---------------------------
// Zstandard (de)compression
// ---------------------------

// DecodeZstdGitBlob reads a zstd-compressed canonical blob and returns Blob.
func DecodeZstdGitBlob(r io.Reader) (*Blob, error) {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("zstd reader: %w", err)
	}
	defer dec.Close()

	raw, err := io.ReadAll(dec)
	if err != nil {
		return nil, fmt.Errorf("read zstd payload: %w", err)
	}

	sep := bytes.IndexByte(raw, 0x00)
	if sep < 0 {
		return nil, fmt.Errorf("invalid object: missing NUL after header")
	}
	header := string(raw[:sep]) // "blob <size>"
	content := raw[sep+1:]

	var objType string
	var size int
	n, err := fmt.Sscanf(header, "%s %d", &objType, &size)
	if err != nil || n != 2 {
		return nil, fmt.Errorf("invalid header %q: %w", header, err)
	}
	if objType != "blob" {
		return nil, fmt.Errorf("unsupported type %q (expected blob)", objType)
	}
	if size > len(content) {
		return nil, fmt.Errorf("truncated content: header size %d > %d bytes read", size, len(content))
	}
	content = content[:size]
	return &Blob{Size: len(content), Content: content}, nil
}

// EncodeZstdGitBlob zstd-compresses canonical blob bytes.
func EncodeZstdGitBlob(content []byte) ([]byte, error) {
	canon := canonicalBlobBytes(content)
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, fmt.Errorf("zstd writer: %w", err)
	}
	if _, err := enc.Write(canon); err != nil {
		return nil, fmt.Errorf("zstd write: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("zstd close: %w", err)
	}
	return buf.Bytes(), nil
}

// ---------------------------
// Conversions (re-hash)
// ---------------------------

type DualDigest struct {
	SHA256 [32]byte
	BLAKE3 [32]byte
	Size   int
}

// DigestsFromZstdGitBlob returns both canonical digests from a zstd blob.
func DigestsFromZstdGitBlob(r io.Reader) (*DualDigest, error) {
	blob, err := DecodeZstdGitBlob(r)
	if err != nil {
		return nil, err
	}
	return &DualDigest{
		SHA256: HashBlobSHA256(blob.Content),
		BLAKE3: HashBlobBLAKE3(blob.Content),
		Size:   blob.Size,
	}, nil
}

// ConvertZstdBlobToBLAKE3 re-hashes content as BLAKE3.
func ConvertZstdBlobToBLAKE3(r io.Reader) (content []byte, blake3Sum [32]byte, err error) {
	blob, err := DecodeZstdGitBlob(r)
	if err != nil {
		return nil, [32]byte{}, err
	}
	return blob.Content, HashBlobBLAKE3(blob.Content), nil
}

// ConvertContentBLAKE3ToSHA256 re-hashes content as SHA-256.
func ConvertContentBLAKE3ToSHA256(content []byte) [32]byte {
	return HashBlobSHA256(content)
}

// ---------------------------
// Small helpers
// ---------------------------

func ReadFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// ---------------------------
// Git object parsing
// ---------------------------

// ParseGitObject reads and decompresses a Git object file, returning the canonical bytes.
func ParseGitObject(objectPath string) ([]byte, error) {
	f, err := os.Open(objectPath)
	if err != nil {
		return nil, fmt.Errorf("open git object %s: %w", objectPath, err)
	}
	defer f.Close()

	// Git objects are zlib-compressed
	zr, err := zlib.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("zlib reader for %s: %w", objectPath, err)
	}
	defer zr.Close()

	return io.ReadAll(zr)
}

// ExtractBlobFromGitObject parses canonical Git object bytes and extracts blob content.
func ExtractBlobFromGitObject(canonical []byte) (*Blob, error) {
	sep := bytes.IndexByte(canonical, 0x00)
	if sep < 0 {
		return nil, fmt.Errorf("invalid git object: missing NUL after header")
	}
	
	header := string(canonical[:sep])
	content := canonical[sep+1:]

	var objType string
	var size int
	n, err := fmt.Sscanf(header, "%s %d", &objType, &size)
	if err != nil || n != 2 {
		return nil, fmt.Errorf("invalid git object header %q: %w", header, err)
	}
	
	if objType != "blob" {
		return nil, fmt.Errorf("not a blob object: %s", objType)
	}
	
	if len(content) != size {
		return nil, fmt.Errorf("content size mismatch: header says %d, got %d", size, len(content))
	}

	return &Blob{Size: size, Content: content}, nil
}

// DiscoverGitObjects walks .git/objects directory and returns paths to all blob objects.
func DiscoverGitObjects(gitDir string) ([]string, error) {
	objectsDir := filepath.Join(gitDir, "objects")
	var objectPaths []string

	err := filepath.Walk(objectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip pack files and other non-loose objects
		if info.IsDir() || strings.Contains(path, "pack") || strings.Contains(path, "info") {
			return nil
		}
		
		// Git loose objects are in subdirs named by first 2 hex chars
		relPath, err := filepath.Rel(objectsDir, path)
		if err != nil {
			return err
		}
		
		// Should be format: "ab/cdef123456..." (40 hex chars total)
		parts := strings.Split(relPath, string(filepath.Separator))
		if len(parts) == 2 && len(parts[0]) == 2 && len(parts[1]) == 38 {
			objectPaths = append(objectPaths, path)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("walking git objects: %w", err)
	}
	
	return objectPaths, nil
}

// ConvertGitBlobToIvaldi converts a Git blob object to Ivaldi format with dual hashes.
func ConvertGitBlobToIvaldi(objectPath string) (*DualDigest, []byte, string, error) {
	canonical, err := ParseGitObject(objectPath)
	if err != nil {
		return nil, nil, "", err
	}
	
	blob, err := ExtractBlobFromGitObject(canonical)
	if err != nil {
		return nil, nil, "", err
	}
	
	// Extract Git SHA1 hash from file path
	// Path format: .git/objects/ab/cdef123456...
	gitSHA1 := extractGitSHA1FromPath(objectPath)
	
	// Generate both hashes for the blob content
	digest := &DualDigest{
		SHA256: HashBlobSHA256(blob.Content),
		BLAKE3: HashBlobBLAKE3(blob.Content),
		Size:   blob.Size,
	}
	
	return digest, blob.Content, gitSHA1, nil
}

// extractGitSHA1FromPath extracts the SHA1 hash from a Git object file path
func extractGitSHA1FromPath(objectPath string) string {
	// Extract from path like .git/objects/ab/cdef123456...
	parts := strings.Split(objectPath, string(filepath.Separator))
	if len(parts) < 2 {
		return ""
	}
	
	// Get the last two components (directory and filename)
	dir := parts[len(parts)-2]
	file := parts[len(parts)-1]
	
	if len(dir) == 2 && len(file) == 38 {
		return dir + file
	}
	
	return ""
}
