package review

import (
	"context"
	"testing"
)

func TestIntegration_FixCycle_PreExistingFindingsNotBlocking(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"spec_alignment": {{Severity: SeverityWarning, File: "handler.go", Description: "missing validation check"}},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"spec_alignment"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Fixed handler",
		SpecContent: "# Spec",
		Cycle:       2,
		PriorFindings: []Finding{
			{Severity: SeverityWarning, File: "handler.go", Description: "missing validation check", Cycle: 1},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.HasBlockingFindings {
		t.Error("pre-existing finding should not trigger blocking on fix cycle")
	}
	if len(result.AllFindings) != 1 {
		t.Errorf("expected 1 finding recorded, got %d", len(result.AllFindings))
	}
	if result.AllFindings[0].Disposition != DispositionPreExisting {
		t.Errorf("disposition should be pre-existing, got %q", result.AllFindings[0].Disposition)
	}
}

func TestIntegration_FixCycle_NewFindingStillBlocks(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {{Severity: SeverityWarning, File: "handler.go", Description: "duplicated error handling"}},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Fixed handler",
		SpecContent: "# Spec",
		Cycle:       2,
		PriorFindings: []Finding{
			{Severity: SeverityError, File: "handler.go", Description: "missing nil check", Cycle: 1},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !result.HasBlockingFindings {
		t.Error("new finding on fix cycle should still block")
	}
	if result.AllFindings[0].Disposition != DispositionNew {
		t.Errorf("disposition should be new, got %q", result.AllFindings[0].Disposition)
	}
}
