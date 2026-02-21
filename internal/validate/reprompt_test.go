package validate

import (
	"strings"
	"testing"
)

func TestBuildReprompt_IncludesViolationsPerFlaggedBead(t *testing.T) {
	candidates := []BeadCandidate{
		{Title: "A", Description: "desc A", AcceptanceCriteria: []string{"a1"}},
		{Title: "B", Description: "desc B", AcceptanceCriteria: []string{"b1"}},
	}
	violations := []Violation{
		{BeadIndex: 1, Rule: "criteria_count", Message: "too many criteria"},
		{BeadIndex: 1, Rule: "scope_signals", Message: "contains scope signal"},
	}

	prompt := BuildReprompt("original prompt", candidates, violations)

	if !strings.Contains(prompt, "- Bead 1:") {
		t.Fatalf("expected flagged bead section for bead 1, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "[criteria_count] too many criteria") {
		t.Fatalf("expected criteria_count violation in prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "[scope_signals] contains scope signal") {
		t.Fatalf("expected scope_signals violation in prompt, got:\n%s", prompt)
	}
}

func TestBuildReprompt_InstructsKeepingUnflaggedBeadsUnchanged(t *testing.T) {
	prompt := BuildReprompt("original prompt", nil, []Violation{{BeadIndex: 0, Rule: "x", Message: "y"}})

	want := "Keep every unflagged bead unchanged"
	if !strings.Contains(prompt, want) {
		t.Fatalf("expected prompt to include %q, got:\n%s", want, prompt)
	}
}

func TestBuildReprompt_RequestsSameJSONFormat(t *testing.T) {
	prompt := BuildReprompt("original prompt", nil, []Violation{{BeadIndex: 0, Rule: "x", Message: "y"}})

	if !strings.Contains(prompt, "Return the same JSON format as before") {
		t.Fatalf("expected instruction to keep same JSON format, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Respond with ONLY the JSON array") {
		t.Fatalf("expected JSON-only response instruction, got:\n%s", prompt)
	}
}
