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

// IsLeafBead returns true when the bead has no downstream dependents.
// A nil DependentCount is treated as 0 (leaf) when dependency metadata
// is not present in the bead payload.
func IsLeafBead(b *Bead) bool {
	return b != nil && (b.DependentCount == nil || *b.DependentCount == 0)
}

// EstimatedFileCount returns the estimated number of files touched by the bead.
// If expected outputs are absent, it returns 0 to indicate unknown.
func EstimatedFileCount(b *Bead) int {
	if b == nil {
		return 0
	}
	return len(b.ExpectedOutputs)
}

// proactiveDecomposeKeywords matches broad-scope keywords as whole words only.
// This prevents false positives on identifiers like "RefactorInvokeFn" or "ExtractArray"
// where the keyword is embedded in a CamelCase name rather than used as a verb/noun.
var proactiveDecomposeKeywords = regexp.MustCompile(
	`(?i)\b(infrastructure|e2e|consolidate|extract|shared|refactor)\b`,
)

var lowComplexityTitlePatterns = regexp.MustCompile(
	`(?i)` +
		`(?:\bmigrate\b.+\bto\b)|` +
		`(?:\bwire\b.+\binto\b)|` +
		`(?:\badd\s+(?:field|config)\b)|` +
		`(?:\bdelete\b)|` +
		`(?:\bdocument\b)|` +
		`(?:\brename\b)|` +
		`(?:\badd\s+t\.parallel\b)|` +
		`(?:\badd\s+compile-time\s+check\b)`,
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

// IsLowComplexityTitle returns true if the title matches one of the
// low-complexity mechanical-work patterns.
func IsLowComplexityTitle(title string) bool {
	t := strings.Join(strings.Fields(title), " ")
	if t == "" {
		return false
	}
	return lowComplexityTitlePatterns.MatchString(t)
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
