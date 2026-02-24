package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// StaticPreambleCacheKey builds a deterministic cache key from static preamble sections.
func StaticPreambleCacheKey(cacheClass string, sections map[string]string) string {
	return StaticPreambleCacheKeyWithExclusions(cacheClass, sections, nil)
}

// StaticPreambleCacheKeyWithExclusions builds a deterministic cache key and omits excluded section names.
func StaticPreambleCacheKeyWithExclusions(cacheClass string, sections map[string]string, exclusions map[string]struct{}) string {
	var b strings.Builder
	b.WriteString(cacheClass)
	canonical := CanonicalizeStaticPreamble(sections, exclusions)
	if canonical != "" {
		b.WriteString("\n")
		b.WriteString(canonical)
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// CanonicalizeStaticPreamble returns a stable section serialization in sorted key order.
func CanonicalizeStaticPreamble(sections map[string]string, exclusions map[string]struct{}) string {
	keys := make([]string, 0, len(sections))
	for key := range sections {
		if _, excluded := exclusions[key]; excluded {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, key := range keys {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(sections[key])
	}
	return b.String()
}
