// Package seals implements the seal naming system for Ivaldi VCS.
//
// This package provides:
// - Generation of unique, memorable seal names from Blake3 hashes
// - 4-word naming pattern: adjective-noun-verb-adverb-hash8
// - Deterministic name generation (same hash = same name)
// - Storage and retrieval of seal names
// - Name validation and conflict resolution
//
// Example seal name: swift-eagle-flies-high-447abe9b
package seals

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// SealName represents a named seal with its metadata
type SealName struct {
	Name      string    // Full name: adjective-noun-verb-adverb-hash8
	Hash      [32]byte  // Full Blake3 hash
	ShortHash string    // First 8 chars of hash (last component of name)
	Timestamp time.Time // When seal was created
	Message   string    // Commit message
}

// Word lists for generating memorable names
var (
	adjectives = []string{
		"swift", "brave", "bold", "clever", "mighty", "gentle", "wise", "noble",
		"fierce", "calm", "bright", "dark", "ancient", "young", "strong", "quick",
		"silent", "loud", "warm", "cool", "sharp", "smooth", "rough", "soft",
		"hard", "light", "heavy", "deep", "shallow", "wide", "narrow", "tall",
		"short", "long", "round", "square", "curved", "straight", "twisted", "pure",
		"wild", "tame", "free", "bound", "open", "closed", "full", "empty",
		"rich", "simple", "complex", "clear", "misty", "bright", "dim", "vivid",
		"pale", "golden", "silver", "crystal", "iron", "steel", "stone", "wooden",
	}

	nouns = []string{
		"eagle", "mountain", "river", "falcon", "wolf", "bear", "storm", "thunder",
		"forest", "ocean", "phoenix", "dragon", "tiger", "lion", "hawk", "raven",
		"fox", "deer", "star", "moon", "sun", "comet", "galaxy", "planet",
		"valley", "peak", "canyon", "meadow", "grove", "spring", "waterfall", "lake",
		"island", "lighthouse", "castle", "tower", "bridge", "gate", "path", "road",
		"sword", "shield", "crown", "gem", "crystal", "flame", "spark", "ember",
		"wind", "wave", "stone", "tree", "flower", "rose", "oak", "pine",
		"marble", "granite", "diamond", "ruby", "sapphire", "emerald", "pearl", "gold",
	}

	verbs = []string{
		"flies", "runs", "leaps", "soars", "dives", "climbs", "swims", "hunts",
		"rests", "guards", "watches", "seeks", "finds", "builds", "grows", "shines",
		"glows", "moves", "stands", "waits", "rises", "falls", "turns", "spins",
		"flows", "burns", "melts", "freezes", "breaks", "heals", "creates", "destroys",
		"protects", "attacks", "defends", "conquers", "explores", "discovers", "reveals", "hides",
		"opens", "closes", "starts", "ends", "begins", "finishes", "travels", "arrives",
		"departs", "returns", "calls", "whispers", "sings", "roars", "echoes", "resonates",
		"reflects", "absorbs", "radiates", "pulsates", "vibrates", "oscillates", "rotates", "revolves",
	}

	adverbs = []string{
		"high", "fast", "slow", "well", "far", "near", "deep", "wide",
		"soft", "hard", "bright", "dark", "quiet", "loud", "free", "true",
		"bold", "wise", "swift", "strong", "gentle", "fierce", "calm", "wild",
		"proud", "humble", "grand", "small", "great", "tiny", "vast", "narrow",
		"smooth", "rough", "sharp", "dull", "clear", "misty", "warm", "cool",
		"hot", "cold", "dry", "wet", "fresh", "stale", "new", "old",
		"young", "ancient", "modern", "classic", "pure", "mixed", "simple", "complex",
		"easy", "hard", "light", "heavy", "quick", "slow", "early", "late",
	}
)

// GenerateSealName creates a unique, memorable name from a Blake3 hash
func GenerateSealName(hash [32]byte) string {
	// Use the hash as a seed to ensure deterministic generation
	seed := binary.LittleEndian.Uint64(hash[:8])

	// Create a seeded random generator for consistent results
	r := rand.New(rand.NewSource(int64(seed)))

	// Select words using the seeded random generator
	adj := adjectives[r.Intn(len(adjectives))]
	noun := nouns[r.Intn(len(nouns))]
	verb := verbs[r.Intn(len(verbs))]
	adv := adverbs[r.Intn(len(adverbs))]

	// Use first 4 bytes of hash for 8-character hex suffix
	shortHash := hex.EncodeToString(hash[:4])

	return fmt.Sprintf("%s-%s-%s-%s-%s", adj, noun, verb, adv, shortHash)
}

// GenerateCustomSealName creates a seal name with user-provided base and hash suffix
func GenerateCustomSealName(customName string, hash [32]byte) string {
	// Sanitize custom name (replace spaces with dashes, lowercase)
	sanitized := strings.ToLower(strings.ReplaceAll(customName, " ", "-"))
	// Remove any non-alphanumeric characters except dashes
	var result strings.Builder
	for _, r := range sanitized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	shortHash := hex.EncodeToString(hash[:4])
	return fmt.Sprintf("%s-%s", result.String(), shortHash)
}

// ParseSealName extracts components from a seal name
func ParseSealName(name string) (adjective, noun, verb, adverb, shortHash string, isValid bool) {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return "", "", "", "", "", false
	}

	// For auto-generated names, expect 5 parts
	if len(parts) == 5 {
		return parts[0], parts[1], parts[2], parts[3], parts[4], true
	}

	// For custom names, the last part should be the hash
	lastPart := parts[len(parts)-1]
	if len(lastPart) == 8 {
		// Check if last part is a valid hex string
		if _, err := hex.DecodeString(lastPart); err == nil {
			baseName := strings.Join(parts[:len(parts)-1], "-")
			return baseName, "", "", "", lastPart, true
		}
	}

	return "", "", "", "", "", false
}

// GetShortHashFromName extracts the 8-character hash from a seal name
func GetShortHashFromName(name string) (string, bool) {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return "", false
	}

	lastPart := parts[len(parts)-1]
	if len(lastPart) == 8 {
		if _, err := hex.DecodeString(lastPart); err == nil {
			return lastPart, true
		}
	}

	return "", false
}

// ValidateSealName checks if a seal name follows the expected format
func ValidateSealName(name string) bool {
	_, _, _, _, _, valid := ParseSealName(name)
	return valid
}

// GetBaseName returns the name without the hash suffix
func GetBaseName(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return name
	}

	lastPart := parts[len(parts)-1]
	if len(lastPart) == 8 {
		if _, err := hex.DecodeString(lastPart); err == nil {
			return strings.Join(parts[:len(parts)-1], "-")
		}
	}

	return name
}

// GenerateTestHash creates a test hash for development purposes
func GenerateTestHash(input string) [32]byte {
	hash := sha256.Sum256([]byte(input))
	var result [32]byte
	copy(result[:], hash[:])
	return result
}

// SealNameGenerator provides methods for creating and managing seal names
type SealNameGenerator struct {
	// Can be extended with configuration options later
}

// NewSealNameGenerator creates a new seal name generator
func NewSealNameGenerator() *SealNameGenerator {
	return &SealNameGenerator{}
}

// Generate creates a new seal name from a hash
func (g *SealNameGenerator) Generate(hash [32]byte) string {
	return GenerateSealName(hash)
}

// GenerateCustom creates a seal name with user-provided base
func (g *SealNameGenerator) GenerateCustom(customName string, hash [32]byte) string {
	return GenerateCustomSealName(customName, hash)
}

// Validate checks if a seal name is valid
func (g *SealNameGenerator) Validate(name string) bool {
	return ValidateSealName(name)
}
