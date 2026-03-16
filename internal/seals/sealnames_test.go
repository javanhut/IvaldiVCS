package seals

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestGenerateSealName_Deterministic(t *testing.T) {
	var hash [32]byte
	copy(hash[:], []byte("deterministic-test-input-value!!"))

	name1 := GenerateSealName(hash)
	name2 := GenerateSealName(hash)
	name3 := GenerateSealName(hash)

	if name1 != name2 {
		t.Errorf("expected identical names for same hash, got %q and %q", name1, name2)
	}
	if name2 != name3 {
		t.Errorf("expected identical names for same hash, got %q and %q", name2, name3)
	}
}

func TestGenerateSealName_DifferentHashes(t *testing.T) {
	var hash1 [32]byte
	var hash2 [32]byte
	hash1[0] = 0x01
	hash2[0] = 0x02

	name1 := GenerateSealName(hash1)
	name2 := GenerateSealName(hash2)

	if name1 == name2 {
		t.Errorf("expected different names for different hashes, both got %q", name1)
	}
}

func TestGenerateSealName_Format(t *testing.T) {
	var hash [32]byte
	for i := range hash {
		hash[i] = byte(i)
	}

	name := GenerateSealName(hash)
	parts := strings.Split(name, "-")

	if len(parts) != 5 {
		t.Errorf("expected 5 parts separated by hyphens, got %d parts: %q", len(parts), name)
	}

	for i, part := range parts {
		if part == "" {
			t.Errorf("part %d is empty in name %q", i, name)
		}
	}
}

func TestGenerateSealName_HexSuffix(t *testing.T) {
	var hash [32]byte
	hash[0] = 0x44
	hash[1] = 0x7a
	hash[2] = 0xbe
	hash[3] = 0x9b

	name := GenerateSealName(hash)
	parts := strings.Split(name, "-")

	if len(parts) != 5 {
		t.Fatalf("expected 5 parts, got %d: %q", len(parts), name)
	}

	suffix := parts[4]
	expectedSuffix := hex.EncodeToString(hash[:4])

	if suffix != expectedSuffix {
		t.Errorf("expected hex suffix %q, got %q", expectedSuffix, suffix)
	}

	if len(suffix) != 8 {
		t.Errorf("expected 8-character hex suffix, got %d characters: %q", len(suffix), suffix)
	}
}

func TestGenerateSealName_ZeroHash(t *testing.T) {
	var hash [32]byte // all zeros

	name := GenerateSealName(hash)

	if name == "" {
		t.Error("expected non-empty name for zero hash")
	}

	parts := strings.Split(name, "-")
	if len(parts) != 5 {
		t.Errorf("expected 5 parts, got %d: %q", len(parts), name)
	}

	expectedSuffix := "00000000"
	if parts[4] != expectedSuffix {
		t.Errorf("expected hex suffix %q for zero hash, got %q", expectedSuffix, parts[4])
	}
}

func TestGenerateSealName_AllOnesHash(t *testing.T) {
	var hash [32]byte
	for i := range hash {
		hash[i] = 0xFF
	}

	name := GenerateSealName(hash)

	if name == "" {
		t.Error("expected non-empty name for all-ones hash")
	}

	parts := strings.Split(name, "-")
	if len(parts) != 5 {
		t.Errorf("expected 5 parts, got %d: %q", len(parts), name)
	}

	expectedSuffix := "ffffffff"
	if parts[4] != expectedSuffix {
		t.Errorf("expected hex suffix %q for all-ones hash, got %q", expectedSuffix, parts[4])
	}
}

func TestGenerateSealName_WordListCoverage(t *testing.T) {
	adjectiveSet := make(map[string]bool)
	nounSet := make(map[string]bool)
	verbSet := make(map[string]bool)
	adverbSet := make(map[string]bool)

	// Generate names from many different hashes
	for i := 0; i < 10000; i++ {
		var hash [32]byte
		hash[0] = byte(i)
		hash[1] = byte(i >> 8)
		hash[2] = byte(i >> 16)
		hash[3] = byte(i >> 24)
		// Vary more bytes for better seed diversity
		hash[4] = byte(i * 7)
		hash[5] = byte(i * 13)
		hash[6] = byte(i * 31)
		hash[7] = byte(i * 37)

		name := GenerateSealName(hash)
		parts := strings.Split(name, "-")
		if len(parts) != 5 {
			t.Fatalf("unexpected format: %q", name)
		}

		adjectiveSet[parts[0]] = true
		nounSet[parts[1]] = true
		verbSet[parts[2]] = true
		adverbSet[parts[3]] = true
	}

	// With 10000 hashes and 64-element lists, we should see good coverage
	minExpected := 30
	if len(adjectiveSet) < minExpected {
		t.Errorf("expected at least %d unique adjectives, got %d", minExpected, len(adjectiveSet))
	}
	if len(nounSet) < minExpected {
		t.Errorf("expected at least %d unique nouns, got %d", minExpected, len(nounSet))
	}
	if len(verbSet) < minExpected {
		t.Errorf("expected at least %d unique verbs, got %d", minExpected, len(verbSet))
	}
	if len(adverbSet) < minExpected {
		t.Errorf("expected at least %d unique adverbs, got %d", minExpected, len(adverbSet))
	}
}

func TestSealName_Struct(t *testing.T) {
	var hash [32]byte
	for i := range hash {
		hash[i] = byte(i * 3)
	}

	now := time.Now()
	name := GenerateSealName(hash)
	shortHash := name[strings.LastIndex(name, "-")+1:]

	seal := SealName{
		Name:      name,
		Hash:      hash,
		ShortHash: shortHash,
		Timestamp: now,
		Message:   "initial seal for testing",
	}

	if seal.Name != name {
		t.Errorf("expected Name %q, got %q", name, seal.Name)
	}
	if seal.Hash != hash {
		t.Errorf("expected Hash to match input hash")
	}
	if seal.ShortHash != shortHash {
		t.Errorf("expected ShortHash %q, got %q", shortHash, seal.ShortHash)
	}
	if !seal.Timestamp.Equal(now) {
		t.Errorf("expected Timestamp %v, got %v", now, seal.Timestamp)
	}
	if seal.Message != "initial seal for testing" {
		t.Errorf("expected Message %q, got %q", "initial seal for testing", seal.Message)
	}

	// Verify ShortHash matches hex encoding of first 4 bytes
	expectedHex := hex.EncodeToString(hash[:4])
	if seal.ShortHash != expectedHex {
		t.Errorf("expected ShortHash to match hex of first 4 hash bytes: want %q, got %q", expectedHex, seal.ShortHash)
	}
}

func BenchmarkGenerateSealName(b *testing.B) {
	var hash [32]byte
	for i := range hash {
		hash[i] = byte(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateSealName(hash)
	}
}
