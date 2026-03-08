package gate

import "strings"

// satisfactionTier returns the LLM tier for the pre-build satisfaction check
// based on bead generation. Gen 0 returns "" (skip). Gen 1 = low (haiku),
// gen 2 = medium (sonnet), gen 3+ = high (opus).
func satisfactionTier(generation int) string {
	switch {
	case generation <= 0:
		return ""
	case generation == 1:
		return "low"
	case generation == 2:
		return "medium"
	default:
		return "high"
	}
}

var structuralKeywords = []string{
	"refactor",
	"reorganize",
	"extract",
	"move",
	"rename",
	"add test",
	"test coverage",
	"integration test",
}

// isStructuralBead returns true if the bead's title or description indicates
// a refactoring or test-only change that should bypass satisfaction checks.
func isStructuralBead(title, description string) bool {
	combined := strings.ToLower(title + " " + description)
	for _, kw := range structuralKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}
