package learnings

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	trigramSize     = 3
	hashPrefixBytes = 8
)

func hashContent(content string) string {
	// Normalize: lowercase, remove extra whitespace.
	normalized := strings.ToLower(strings.TrimSpace(content))
	normalized = whitespaceRegex.ReplaceAllString(normalized, " ")

	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:hashPrefixBytes]) // First 8 bytes is enough.
}

// similarity calculates a simple trigram-based similarity score.
func similarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))

	if a == b {
		return 1.0
	}

	trigramsA := trigrams(a)
	trigramsB := trigrams(b)

	if len(trigramsA) == 0 || len(trigramsB) == 0 {
		return 0.0
	}

	// Count matching trigrams.
	matches := 0
	for t := range trigramsA {
		if trigramsB[t] {
			matches++
		}
	}

	// Jaccard similarity.
	union := len(trigramsA) + len(trigramsB) - matches
	if union == 0 {
		return 0.0
	}
	return float64(matches) / float64(union)
}

func trigrams(s string) map[string]bool {
	result := make(map[string]bool)
	if len(s) < trigramSize {
		return result
	}
	for i := 0; i <= len(s)-trigramSize; i++ {
		result[s[i:i+trigramSize]] = true
	}
	return result
}
