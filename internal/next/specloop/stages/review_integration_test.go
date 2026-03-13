package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestIntegration_ReviewBlocksReadyForReview(t *testing.T) {
	// Build a minimal pipeline: validate (pass) -> review (blocking finding) -> finalize
	validateStage := &passStage{name: "validate"}
	reviewRunner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
			},
			BlockingFindings: []review.Finding{
				{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
			},
			HasBlockingFindings: true,
			FindingsByFacet: map[string][]review.Finding{
				"spec_alignment": {{Severity: review.SeverityError, File: "handler.go", Description: "missing validation"}},
			},
		},
	}

	reviewStage := NewReviewStage(reviewRunner, ReviewStageConfig{}, nil)
	planStage := &passStage{name: "plan"}

	stages := []specloop.Stage{planStage, validateStage, reviewStage}

	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
	})

	rs := runstore.NewRunState("test-spec", "test-project")
	if err := loop.Run(context.Background(), rs); err != nil {
		t.Fatalf("loop.Run: %v", err)
	}

	if rs.Status == runstore.StatusReadyForReview {
		t.Error("review blocking findings should prevent ready_for_review")
	}
	if rs.Status != runstore.StatusNeedsHuman {
		t.Errorf("expected needs_human when budget exhausted with blocking findings, got %q", rs.Status)
	}
}

type passStage struct {
	name string
}

func (s *passStage) Name() string { return s.name }
func (s *passStage) Run(_ context.Context, _ *runstore.RunState) (specloop.NextAction, error) {
	return specloop.NextAction{Kind: specloop.Continue}, nil
}
