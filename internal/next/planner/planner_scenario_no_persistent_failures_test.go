package planner

import (
	"strings"
	"testing"
)

func TestScenario_NoPersistentFailures_PromptUnchanged(t *testing.T) {
	// Seed: a fix plan request with only ordinary failures, no persistent-failure: entries
	req := FixPlanRequest{
		OriginalPlan: Plan{SpecID: "s1", Cycle: 1},
		CompletedTasks: []CompletedTask{{
			TaskID:            "t-001",
			Attempts:          1,
			FilesChanged:      []string{"pkg/foo.go"},
			ValidationOutcome: "failed",
		}},
		Failures: []string{
			"contract:scenario-contracts.yaml TestAdd expected 4 got 5",
			"go test ./pkg/... FAIL",
			"lint error in pkg/foo.go: unused variable",
		},
		CurrentDiff: "diff --git a/pkg/foo.go b/pkg/foo.go\n-old\n+new",
		Cycle:       2,
	}

	// Invoke
	prompt := buildFixPlanPrompt(req)

	// Assert: no Persistent Failures section appears
	if strings.Contains(prompt, "## Persistent Failures") {
		t.Fatal("prompt must not contain '## Persistent Failures' section when no persistent-failure: entries exist")
	}
	if strings.Contains(prompt, "Possible Bad Contracts") {
		t.Fatal("prompt must not contain 'Possible Bad Contracts' when no persistent-failure: entries exist")
	}
	if strings.Contains(prompt, "BEFORE creating any implementation fix task") {
		t.Error("no persistent failures — audit instruction block must not appear")
	}

	// Assert: standard sections are present
	if !strings.Contains(prompt, "## Completed Tasks") {
		t.Fatal("prompt must contain Completed Tasks section")
	}
	if !strings.Contains(prompt, "## Validation Failures to Fix") {
		t.Fatal("prompt must contain Validation Failures section")
	}
	if !strings.Contains(prompt, "## Current Diff") {
		t.Fatal("prompt must contain Current Diff section")
	}
	if !strings.Contains(prompt, "## Instructions") {
		t.Fatal("prompt must contain Instructions section")
	}

	// Assert: all ordinary failures appear in the validation section
	for _, f := range req.Failures {
		if !strings.Contains(prompt, f) {
			t.Fatalf("prompt must contain failure %q", f)
		}
	}
}