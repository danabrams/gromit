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
