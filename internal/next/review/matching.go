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

// descriptionMatches returns true if either description contains the other as a substring.
// Empty descriptions never match to prevent silently suppressing findings.
func descriptionMatches(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}
