package review

import "strings"

// LabelDispositions returns a copy of current findings with Disposition set.
// A finding is pre-existing if it matches a prior finding by file path AND
// substring match on description (either direction). Otherwise it is new.
func LabelDispositions(current, prior []Finding) []Finding {
	result := make([]Finding, len(current))
	copy(result, current)

	for i := range result {
		result[i].Disposition = DispositionNew
		for _, p := range prior {
			if result[i].File == p.File && descriptionMatches(result[i].Description, p.Description) {
				result[i].Disposition = DispositionPreExisting
				break
			}
		}
	}
	return result
}

// minSubstringMatchLen is the minimum description length for substring matching.
// Descriptions shorter than this threshold require an exact match to avoid
// false positives where short strings like "nil" or "error" match unrelated findings.
const minSubstringMatchLen = 10

// descriptionMatches returns true if either description contains the other as a substring.
// Empty descriptions never match to prevent silently suppressing findings.
// Short descriptions (below minSubstringMatchLen) require exact match only.
// Comparison is case-insensitive because LLM-generated descriptions have
// inconsistent casing across cycles.
func descriptionMatches(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	lowerA := strings.ToLower(a)
	lowerB := strings.ToLower(b)
	if len(a) < minSubstringMatchLen || len(b) < minSubstringMatchLen {
		return lowerA == lowerB
	}
	return strings.Contains(lowerA, lowerB) || strings.Contains(lowerB, lowerA)
}
