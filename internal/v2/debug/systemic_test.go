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

func TestBuildSystemicRecommendation_IncludesRationaleAndApproval(t *testing.T) {
	signal := "Prompt fragment is ambiguous and needs clarity."
	recommendation := BuildSystemicRecommendation(RootCauseUnclearBead, signal)

	if recommendation == "" {
		t.Fatal("recommendation = empty, want non-empty")
	}
	if !strings.Contains(recommendation, "Rationale:") {
		t.Fatalf("recommendation = %q, want rationale details", recommendation)
	}
	if !strings.Contains(recommendation, "Awaiting human approval") {
		t.Fatalf("recommendation = %q, want human approval reminder", recommendation)
	}
	if !strings.Contains(recommendation, signal) {
		t.Fatalf("recommendation = %q, want failure signal reflected", recommendation)
	}
}

func TestDetectSystemicCategory(t *testing.T) {
	tests := []struct {
		name      string
		rootCause RootCause
		signal    string
		want      string
	}{
		{name: "unclear bead root cause", rootCause: RootCauseUnclearBead, want: "prompt"},
		{name: "bad decomposition root cause", rootCause: RootCauseBadDecomposition, want: "process"},
		{name: "signal contains guard", signal: "Add a pipeline guard here", want: "guard"},
		{name: "signal contains rules file", signal: "Update RULES.md to clarify process rules", want: "rule"},
		{name: "no systemic match", rootCause: RootCauseFlakyTest, signal: "transient failure", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectSystemicCategory(tc.rootCause, tc.signal); got != tc.want {
				t.Fatalf("DetectSystemicCategory(%q, %q) = %q, want %q", tc.rootCause, tc.signal, got, tc.want)
			}
		})
	}
}
