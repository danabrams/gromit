package review

import (
	"context"
	"testing"
)

func TestIntegration_EnableFacetViaConfig(t *testing.T) {
	agent := &facetCapturingAgent{
		reviewedFacets: make(map[string]bool),
	}

	runner := NewRunner(agent, RunnerConfig{
		Facets:    []string{"spec_alignment", "code_quality", "logic_gaps"},
		Threshold: SeveritySuggestion,
	})

	_, err := runner.Run(context.Background(), RunInput{
		DiffSummary: "Added handler",
		SpecContent: "# Spec",
		Cycle:       1,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, facet := range []string{"spec_alignment", "code_quality", "logic_gaps"} {
		if !agent.reviewedFacets[facet] {
			t.Errorf("facet %q should have been reviewed", facet)
		}
	}
}

type facetCapturingAgent struct {
	reviewedFacets map[string]bool
}

func (a *facetCapturingAgent) ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error) {
	a.reviewedFacets[facetName] = true
	return nil, nil
}
