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
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
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

