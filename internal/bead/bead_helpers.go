package bead

import (
	"regexp"
	"strings"
)

const (
	specLabelPrefix          = "spec:"
	proactiveStructThreshold = 3
)

var testOnlyTitlePrefixes = []string{
	"add tests for",
	"add unit tests for",
	"add acceptance tests for",
	"add integration tests for",
	"write tests for",
	"write unit tests for",
	"write acceptance tests for",
	"write integration tests for",
}

// FindSpecLabel returns the spec name from labels (spec:<name>) or empty string.
func FindSpecLabel(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, specLabelPrefix) {
			return strings.TrimPrefix(label, specLabelPrefix)
		}
	}
	return ""
}

// HasLabel checks if a bead has a specific label.
func HasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

// IsTestOnlyBead returns true if the bead's title indicates that tests ARE the deliverable
// (e.g., "Add unit tests for X", "Write tests for Y"). Such beads should skip the ATDD
// pre-pass since acceptance tests are the implementation itself.
func IsTestOnlyBead(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	if t == "" {
		return false
	}
	for _, prefix := range testOnlyTitlePrefixes {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

// proactiveDecomposeKeywords matches broad-scope keywords as whole words only.
// This prevents false positives on identifiers like "RefactorInvokeFn" or "ExtractArray"
// where the keyword is embedded in a CamelCase name rather than used as a verb/noun.
var proactiveDecomposeKeywords = regexp.MustCompile(
	`(?i)\b(infrastructure|e2e|consolidate|extract|shared|refactor)\b`,
)

// IsProactiveDecompositionCandidate returns true if the bead's title contains keywords
// that signal broad scope and should trigger proactive decomposition before first attempt.
// Keywords must appear as whole words - "Refactor the auth system" matches but
// "Update RefactorInvokeFn" does not.
func IsProactiveDecompositionCandidate(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return false
	}
	return proactiveDecomposeKeywords.MatchString(t)
}

// IsProactiveDecompositionCandidateWithDesc returns true if the bead should be proactively
// decomposed before first attempt, based on title keywords OR a description that mentions
// "struct" 3+ times (used as a proxy for introducing 3+ new type definitions).
func IsProactiveDecompositionCandidateWithDesc(title, description string) bool {
	if IsProactiveDecompositionCandidate(title) {
		return true
	}
	// Count "struct" occurrences in description as a proxy for new type definitions.
	count := strings.Count(strings.ToLower(description), "struct")
	return count >= proactiveStructThreshold
}

// IsMethodologyActive checks if a methodology (e.g., "atdd", "tdd") is active for a bead.
// It checks for a label like "atdd:true" or "atdd:false" and returns that value if present.
// If no matching label is found, it falls back to the globalDefault value.
func IsMethodologyActive(labels []string, methodologyName string, globalDefault bool) bool {
	trueLabel := methodologyName + ":true"
	falseLabel := methodologyName + ":false"

	for _, label := range labels {
		if label == trueLabel {
			return true
		}
		if label == falseLabel {
			return false
		}
	}

	return globalDefault
}
