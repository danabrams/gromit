package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// StaticPreambleCacheKey builds a deterministic cache key from static preamble sections.
func StaticPreambleCacheKey(cacheClass string, sections map[string]string) string {
	keys := make([]string, 0, len(sections))
	for key := range sections {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(cacheClass)
	for _, key := range keys {
		b.WriteString("\n")
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(sections[key])
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
