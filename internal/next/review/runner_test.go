package review

import (
	"context"
	"fmt"
	"testing"
)

// mockReviewAgent is a test double for the LLM review agent.
type mockReviewAgent struct {
	findings map[string][]Finding // facet name -> findings
	errors   map[string]error     // facet name -> error (optional)
}

func (m *mockReviewAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	if m.errors != nil {
		if err, ok := m.errors[facetName]; ok {
			return nil, err
		}
	}
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

// programmableReviewAgent allows per-call control of ReviewFacet behavior.
type programmableReviewAgent struct {
	reviewFn func(ctx context.Context, facetName string, prompt string) ([]Finding, error)
}

func (a *programmableReviewAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	return a.reviewFn(ctx, facetName, prompt)
}

func TestRunner_FacetRetry_OnUnparseableJSON(t *testing.T) {
	callCount := 0
	agent := &programmableReviewAgent{
		reviewFn: func(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
			callCount++
			if callCount == 1 {
				return nil, &ParseError{Msg: "invalid JSON in LLM response"}
			}
			return []Finding{{Severity: SeverityWarning, File: "handler.go", Description: "missing check"}}, nil
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:       []string{"code_quality"},
		Threshold:    SeverityWarning,
		FacetRetries: 2,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Modified handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", callCount)
	}
	if len(result.AllFindings) != 1 {
		t.Errorf("expected 1 finding after successful retry, got %d", len(result.AllFindings))
	}
	if result.ErroredFacets["code_quality"] != "" {
		t.Error("facet should not be errored after successful retry")
	}
}

func TestRunner_FacetRetry_ExhaustionMarksErrored(t *testing.T) {
	agent := &programmableReviewAgent{
		reviewFn: func(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
			return nil, &ParseError{Msg: "invalid JSON in LLM response"}
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:       []string{"code_quality"},
		Threshold:    SeverityWarning,
		FacetRetries: 2,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Modified handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run should not return top-level error: %v", err)
	}

	if result.ErroredFacets["code_quality"] == "" {
		t.Error("facet should be marked errored after retry exhaustion")
	}
}

func TestRunner_FacetRetry_MissingFields(t *testing.T) {
	callCount := 0
	agent := &programmableReviewAgent{
		reviewFn: func(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
			callCount++
			if callCount == 1 {
				return nil, &ParseError{Msg: "missing required field: severity"}
			}
			return []Finding{{Severity: SeverityInfo, File: "main.go", Description: "ok"}}, nil
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:       []string{"spec_alignment"},
		Threshold:    SeverityWarning,
		FacetRetries: 3,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Changed main",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
	if len(result.AllFindings) != 1 {
		t.Errorf("expected 1 finding after retry, got %d", len(result.AllFindings))
	}
}

func TestRunner_AllFacetsErrored(t *testing.T) {
	agent := &programmableReviewAgent{
		reviewFn: func(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
			return nil, &ParseError{Msg: "bad response"}
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:       []string{"spec_alignment", "code_quality"},
		Threshold:    SeverityWarning,
		FacetRetries: 1,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Some diff",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(result.ErroredFacets) != 2 {
		t.Errorf("expected 2 errored facets, got %d", len(result.ErroredFacets))
	}
	if len(result.AllFindings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.AllFindings))
	}
}

func TestRunner_FacetError_ContinuesWithOtherFacets(t *testing.T) {
	agent := &mockReviewAgent{
		findings: map[string][]Finding{
			"spec_alignment": {{Severity: SeverityError, File: "handler.go", Description: "missing validation"}},
		},
		errors: map[string]error{
			"code_quality": fmt.Errorf("API timeout"),
		},
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"spec_alignment", "code_quality"},
		Threshold: SeverityWarning,
	})

	result, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run should not return error on partial facet failure: %v", err)
	}

	if len(result.AllFindings) != 1 {
		t.Errorf("expected 1 finding from successful facet, got %d", len(result.AllFindings))
	}

	if result.ErroredFacets == nil || result.ErroredFacets["code_quality"] == "" {
		t.Error("expected code_quality to be marked as errored")
	}
}

func TestRunResult_NormalizeNilFields(t *testing.T) {
	r := RunResult{}
	r.NormalizeNilFields()
	if r.AllFindings == nil {
		t.Error("AllFindings should not be nil after normalize")
	}
	if r.BlockingFindings == nil {
		t.Error("BlockingFindings should not be nil after normalize")
	}
	if r.FindingsByFacet == nil {
		t.Error("FindingsByFacet should not be nil after normalize")
	}
	if r.ErroredFacets == nil {
		t.Error("ErroredFacets should not be nil after normalize")
	}
}
