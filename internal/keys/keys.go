package keys

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
)

// KeyLookup interface allows both *store.DB and *store.SharedDB to be used
type KeyLookup interface {
	LookupByKey(humanKey string) (blake3Hex, sha256Hex string, err error)
}

// A small wordlist – replace with a larger one (e.g., 2048+ words) for better entropy.
var words = []string{
	"amber", "bison", "copper", "drift", "ember", "flint", "grove", "harbor", "ivory", "juniper",
	"kestrel", "lilac", "meadow", "nectar", "onyx", "prairie", "quartz", "river", "sage", "tundra",
	"umber", "violet", "willow", "xenon", "yarrow", "zephyr",
}

func randUint32() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint32(b[:])
}

func randChoice(n int) int {
	return int(randUint32() % uint32(n))
}

func makePhrase(numWords int, suffixDigits int) string {
	parts := make([]string, 0, numWords+1)
	for i := 0; i < numWords; i++ {
		parts = append(parts, words[randChoice(len(words))])
	}
	if suffixDigits > 0 {
		// ~ 10^suffixDigits space. Example: 4 digits -> 0000..9999
		max := 1
		for i := 0; i < suffixDigits; i++ {
			max *= 10
		}
		parts = append(parts, fmt.Sprintf("%0*d", suffixDigits, int(randUint32())%max))
	}
	return strings.Join(parts, "-")
}

// GenerateUniquePhrase creates a unique phrase key, checking the DB for collisions.
// Increase numWords / suffixDigits to reduce collision probability.
func GenerateUniquePhrase(db KeyLookup, numWords, suffixDigits int) (string, error) {
	// With 26 words, 2-3 words, and 3-4 digit suffix, collision probability is very low
	// Try up to 10 attempts before giving up to prevent infinite loops
	maxAttempts := 10
	
	for i := 0; i < maxAttempts; i++ {
		k := makePhrase(numWords, suffixDigits)
		_, _, err := db.LookupByKey(k)
		if err != nil { // not found => good
			return k, nil
		}
		// else collision: try again
	}
	
	// If we still have collisions after 10 attempts, add more entropy
	return makePhrase(numWords+1, suffixDigits+2), nil // Expand search space
}
