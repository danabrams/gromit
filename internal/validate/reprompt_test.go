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

func TestBuildReprompt_RequestsExpectedOutputsContractFields(t *testing.T) {
	prompt := BuildReprompt("original prompt", nil, []Violation{{BeadIndex: 0, Rule: "x", Message: "y"}})

	if !strings.Contains(prompt, "expected_outputs") {
		t.Fatalf("expected reprompt to require expected_outputs field, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "depends_on_index") {
		t.Fatalf("expected reprompt to require depends_on_index field, got:\n%s", prompt)
	}
}

func TestBuildReprompt_RendersCandidateDependencyAndExpectedOutputsContext(t *testing.T) {
	candidates := []BeadCandidate{
		{
			Title:              "Implement auth API",
			Description:        "Add auth endpoint wiring",
			DependsOnIndex:     []int{0, 2},
			AcceptanceCriteria: []string{"Auth endpoint responds with 200"},
			ExpectedOutputs:    []string{"Auth handler created", "Route registered"},
		},
	}

	prompt := BuildReprompt("original prompt", candidates, []Violation{{BeadIndex: 0, Rule: "x", Message: "y"}})

	if !strings.Contains(prompt, "Depends On Index: [0, 2]") {
		t.Fatalf("expected candidate context to include depends_on_index data, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Expected Outputs:") {
		t.Fatalf("expected candidate context to include expected_outputs section, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Auth handler created") {
		t.Fatalf("expected candidate context to include expected_outputs values, got:\n%s", prompt)
	}
}

func TestBuildComplexityRepromptFeedback_IncludesAllCandidateReasons(t *testing.T) {
	feedback := BuildComplexityRepromptFeedback([]CandidateComplexityResult{
		{
			Title: "Split auth orchestration",
			Reasons: []string{
				"estimated_files=8 crosses the high-complexity threshold",
				"acceptance criteria mixes API wiring and data migration",
			},
		},
		{
			Title: "Refine CLI output",
			Reasons: []string{
				"touches multiple unrelated packages",
			},
		},
	})

	if !strings.Contains(feedback, "Split auth orchestration") {
		t.Fatalf("feedback missing first candidate title, got:\n%s", feedback)
	}
	if !strings.Contains(feedback, "estimated_files=8 crosses the high-complexity threshold") {
		t.Fatalf("feedback missing first candidate reason, got:\n%s", feedback)
	}
	if !strings.Contains(feedback, "acceptance criteria mixes API wiring and data migration") {
		t.Fatalf("feedback missing second reason for first candidate, got:\n%s", feedback)
	}
	if !strings.Contains(feedback, "Refine CLI output") {
		t.Fatalf("feedback missing second candidate title, got:\n%s", feedback)
	}
	if !strings.Contains(feedback, "touches multiple unrelated packages") {
		t.Fatalf("feedback missing second candidate reason, got:\n%s", feedback)
	}
}

func TestBuildComplexityRepromptFeedback_UsesStructuredRepromptCompositionPattern(t *testing.T) {
	feedback := BuildComplexityRepromptFeedback([]CandidateComplexityResult{
		{
			Title:   "Split auth orchestration",
			Reasons: []string{"estimated_files=8 crosses the high-complexity threshold"},
		},
	})

	requiredSections := []string{
		"## Complexity Feedback",
		"### Split-Concerns Guidance",
		"### Reduce-Breadth Guidance",
		"### Preserve-Semantics Guidance",
		"### Avoid-Overlap Guidance",
	}

	for _, section := range requiredSections {
		if !strings.Contains(feedback, section) {
			t.Fatalf("feedback missing section %q, got:\n%s", section, feedback)
		}
	}
}

func TestBuildComplexityRepromptFeedback_CitesCandidateOnEachReasonLine(t *testing.T) {
	feedback := BuildComplexityRepromptFeedback([]CandidateComplexityResult{
		{
			Title:   "Split auth orchestration",
			Reasons: []string{"estimated_files=8 crosses the high-complexity threshold"},
		},
	})

	if !strings.Contains(feedback, "reason [candidate: Split auth orchestration]: estimated_files=8 crosses the high-complexity threshold") {
		t.Fatalf("feedback missing candidate citation on reason line, got:\n%s", feedback)
	}
}

func TestBuildComplexityReprompt_EmptyInputReturnsEmptyOutput(t *testing.T) {
	if got := BuildComplexityReprompt(nil); got != "" {
		t.Fatalf("BuildComplexityReprompt(nil) = %q, want empty string", got)
	}
}
