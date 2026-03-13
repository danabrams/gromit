package stages

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type mockReviewRunner struct {
	result *review.RunResult
	err    error
}

func (m *mockReviewRunner) Run(ctx context.Context, input review.RunInput) (*review.RunResult, error) {
	return m.result, m.err
}

type fakeDiffProvider struct {
	diff string
	err  error
}

func (f *fakeDiffProvider) Diff(baseBranch string) (string, error) {
	return f.diff, f.err
}

func TestReviewStage_Name(t *testing.T) {
	s := NewReviewStage(nil, ReviewStageConfig{}, nil)
	if s.Name() != "review" {
		t.Errorf("Name() = %q, want %q", s.Name(), "review")
	}
}

func TestReviewStage_Clean_Continue(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings:         []review.Finding{},
			BlockingFindings:    []review.Finding{},
			HasBlockingFindings: false,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{diff: "some diff"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalReviewPassed {
		t.Error("expected FinalReviewPassed = true")
	}
}

func TestReviewStage_BlockingFindings_ReplanFrom(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
			},
			BlockingFindings: []review.Finding{
				{Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
			},
			HasBlockingFindings: true,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{diff: "some diff"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext")
	}
	if len(action.Context.Failures) == 0 {
		t.Error("expected at least one failure message")
	}
	if rs.FinalReviewPassed {
		t.Error("expected FinalReviewPassed = false")
	}
	if len(rs.ReviewFindings) == 0 {
		t.Error("expected ReviewFindings to be populated")
	}
}

func TestReviewStage_InfoOnly_Continue(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Severity: review.SeverityInfo, File: "handler.go", Description: "consider helper"},
			},
			BlockingFindings:    []review.Finding{},
			HasBlockingFindings: false,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{diff: "some diff"},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("info-only findings should Continue, got %v", action.Kind)
	}
}

func TestReviewStage_StoresFindingsInRunState(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Facet: "code_quality", Severity: review.SeverityInfo, File: "handler.go", Description: "consider helper"},
			},
			FindingsByFacet: map[string][]review.Finding{
				"code_quality": {{Severity: review.SeverityInfo, File: "handler.go", Description: "consider helper"}},
			},
			BlockingFindings:    []review.Finding{},
			HasBlockingFindings: false,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(rs.ReviewFindings) == 0 {
		t.Fatal("ReviewFindings should be populated in RunState after review")
	}
}

type capturingReviewRunner struct {
	resultFn func() *review.RunResult
	capture  func(review.RunInput)
}

func (c *capturingReviewRunner) Run(ctx context.Context, input review.RunInput) (*review.RunResult, error) {
	if c.capture != nil {
		c.capture(input)
	}
	return c.resultFn(), nil
}

func TestReviewStage_FixCycle_PassesPriorFindings(t *testing.T) {
	cycle1Findings := []review.Finding{
		{Facet: "spec_alignment", Severity: review.SeverityWarning, File: "handler.go", Description: "missing check"},
	}

	var capturedInput review.RunInput
	callCount := 0
	runner := &capturingReviewRunner{
		resultFn: func() *review.RunResult {
			callCount++
			if callCount == 1 {
				return &review.RunResult{
					AllFindings:         cycle1Findings,
					BlockingFindings:    []review.Finding{},
					HasBlockingFindings: false,
				}
			}
			return &review.RunResult{
				AllFindings:         []review.Finding{},
				BlockingFindings:    []review.Finding{},
				HasBlockingFindings: false,
			}
		},
		capture: func(input review.RunInput) {
			capturedInput = input
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)

	// Cycle 1: produces findings, stage stores them internally
	rs1 := runstore.NewRunState("test-spec", "test-project")
	rs1.Cycle = 1
	stage.Run(context.Background(), rs1)

	// Cycle 2: stage should pass prior findings from its internal state
	rs2 := runstore.NewRunState("test-spec", "test-project")
	rs2.Cycle = 2
	_, err := stage.Run(context.Background(), rs2)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(capturedInput.PriorFindings) == 0 {
		t.Error("on fix cycle, prior findings should be passed to runner from stage-local state")
	}
	if capturedInput.PriorFindings[0].Facet != "spec_alignment" {
		t.Errorf("expected spec_alignment facet, got %s", capturedInput.PriorFindings[0].Facet)
	}
}

func TestReviewStage_DiffProviderError(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{err: errors.New("git not found")},
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err == nil {
		t.Fatal("expected error from DiffProvider failure")
	}
	if !strings.Contains(err.Error(), "review diff") {
		t.Errorf("expected wrapped error with 'review diff', got: %v", err)
	}
}
