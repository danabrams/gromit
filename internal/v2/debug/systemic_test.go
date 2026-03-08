package debug

import (
	"strings"
	"testing"
)

func TestBuildSystemicRecommendation_UsesSystemicKeywords(t *testing.T) {
	recommendation := BuildSystemicRecommendation(
		RootCauseBadBuildOutput,
		"Add a pipeline code guard and RULES.md update to prevent this failure class.",
	)

	if recommendation == "" {
		t.Fatal("recommendation = empty, want non-empty")
	}
	if !strings.Contains(strings.ToLower(recommendation), "human review") {
		t.Fatalf("recommendation = %q, want human-review guidance", recommendation)
	}
	if !strings.Contains(strings.ToLower(recommendation), "guard") {
		t.Fatalf("recommendation = %q, want guard guidance", recommendation)
	}
}
