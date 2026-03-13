package review

import (
	"context"
	"testing"
)

// mockReviewAgent is a test double for the LLM review agent.
type mockReviewAgent struct {
	findings map[string][]Finding // facet name -> findings
}

func (m *mockReviewAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	return m.findings[facetName], nil
}

func TestRunner_RunAllFacets(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"spec_alignment": {{Severity: SeverityError, File: "handler.go", Description: "missing validation"}},
			"code_quality":   {},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"spec_alignment", "code_quality"},
		Threshold: SeveritySuggestion,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(result.AllFindings) != 1 {
		t.Fatalf("expected 1 finding total, got %d", len(result.AllFindings))
	}
	if len(result.BlockingFindings) != 1 {
		t.Fatalf("expected 1 blocking finding, got %d", len(result.BlockingFindings))
	}
	if !result.HasBlockingFindings {
		t.Error("HasBlockingFindings should be true")
	}
}

func TestRunner_InfoFindingsNeverBlock(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"code_quality": {{Severity: SeverityInfo, File: "handler.go", Description: "consider extracting helper"}},
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"code_quality"},
		Threshold: SeveritySuggestion,
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
		t.Error("info findings should not block")
	}
	if len(result.AllFindings) != 1 {
		t.Error("info finding should still appear in AllFindings")
	}
}

func TestRunner_FixCycle_LabelsDispositions(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"spec_alignment": {{Severity: SeverityWarning, File: "handler.go", Description: "missing validation"}},
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
			{Severity: SeverityWarning, File: "handler.go", Description: "missing validation"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The finding matches a prior finding, so it should be pre-existing
	if result.AllFindings[0].Disposition != DispositionPreExisting {
		t.Errorf("expected pre-existing disposition, got %q", result.AllFindings[0].Disposition)
	}
	// Pre-existing findings should NOT block
	if result.HasBlockingFindings {
		t.Error("pre-existing findings should not trigger blocking")
	}
}
