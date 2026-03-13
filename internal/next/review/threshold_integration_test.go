package review

import (
	"context"
	"testing"
)

func TestIntegration_ThresholdError_WarningsNonBlocking(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {
				{Severity: SeverityWarning, File: "handler.go", Description: "duplicate logic"},
				{Severity: SeverityWarning, File: "router.go", Description: "long function"},
			},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeverityError,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.HasBlockingFindings {
		t.Error("warnings should NOT block at error threshold")
	}
	if len(result.AllFindings) != 2 {
		t.Errorf("all findings should still be recorded, got %d", len(result.AllFindings))
	}
}

func TestIntegration_ThresholdWarning_WarningsBlock(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {
				{Severity: SeverityWarning, File: "handler.go", Description: "duplicate logic"},
			},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeverityWarning,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !result.HasBlockingFindings {
		t.Error("warnings SHOULD block at warning threshold (default)")
	}
}

func TestIntegration_ThresholdWarning_SuggestionsNonBlocking(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {
				{Severity: SeveritySuggestion, File: "handler.go", Description: "consider extracting helper"},
			},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeverityWarning,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.HasBlockingFindings {
		t.Error("suggestions should NOT block at warning threshold (default)")
	}
	if len(result.AllFindings) != 1 {
		t.Error("suggestion should still be recorded in AllFindings")
	}
}
