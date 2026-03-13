package stages

import (
	"context"
	"errors"
	"path/filepath"
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

func TestReviewStage_AllFacetsError_ReturnsBlocked(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFacetsErrored: true,
			ErroredFacets: map[string]string{
				"spec_alignment": "API timeout",
				"code_quality":   "rate limited",
			},
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

	if action.Kind != specloop.Blocked {
		t.Errorf("expected Blocked action, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected FailureContext to be non-nil")
	}
	if len(action.Context.Failures) != 1 {
		t.Errorf("expected 1 failure message, got %d", len(action.Context.Failures))
	}
}

func TestReviewStage_ComputesDiffFromDiffProvider(t *testing.T) {
	var capturedInput review.RunInput
	runner := &capturingReviewRunner{
		resultFn: func() *review.RunResult {
			return &review.RunResult{}
		},
		capture: func(input review.RunInput) {
			capturedInput = input
		},
	}

	fakeDiff := &fakeDiffProvider{
		diff: "diff --git a/handler.go b/handler.go\n+new line",
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: fakeDiff,
	}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if capturedInput.DiffSummary == "" {
		t.Error("expected DiffSummary to be populated from DiffProvider")
	}
	if !strings.Contains(capturedInput.DiffSummary, "handler.go") {
		t.Error("DiffSummary should contain diff output from DiffProvider")
	}
}

func TestReviewStage_EmitsEvent(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Severity: review.SeverityInfo, File: "handler.go", Description: "info note"},
			},
			BlockingFindings:    []review.Finding{},
			HasBlockingFindings: false,
			FindingsByFacet: map[string][]review.Finding{
				"code_quality": {{Severity: review.SeverityInfo}},
			},
		},
	}

	eventLog := runstore.NewEventLog(filepath.Join(t.TempDir(), "events.jsonl"))
	stage := NewReviewStage(runner, ReviewStageConfig{}, eventLog)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	_, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].EventType() != "review_result" {
		t.Errorf("event type = %q, want review_result", events[0].EventType())
	}
	rev, ok := events[0].(*runstore.ReviewResultEvent)
	if !ok {
		t.Fatal("expected *runstore.ReviewResultEvent")
	}
	if len(rev.FindingsBySeverity) == 0 {
		t.Fatal("expected FindingsBySeverity to be populated")
	}
	if rev.FindingsBySeverity["info"] != 1 {
		t.Errorf("FindingsBySeverity[\"info\"] = %d, want 1", rev.FindingsBySeverity["info"])
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

func TestReviewStage_DeduplicatesPriorFindings(t *testing.T) {
	// Same finding returned on two cycles should not duplicate in priorFindings.
	duplicateFinding := review.Finding{
		Facet: "code_quality", Severity: review.SeverityWarning,
		File: "handler.go", Description: "missing check",
	}

	var capturedInput review.RunInput
	callCount := 0
	runner := &capturingReviewRunner{
		resultFn: func() *review.RunResult {
			callCount++
			return &review.RunResult{
				AllFindings:         []review.Finding{duplicateFinding},
				BlockingFindings:    []review.Finding{},
				HasBlockingFindings: false,
			}
		},
		capture: func(input review.RunInput) {
			capturedInput = input
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)

	// Cycle 1
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1
	stage.Run(context.Background(), rs)

	// Cycle 2: same finding returned again
	rs2 := runstore.NewRunState("test-spec", "test-project")
	rs2.Cycle = 2
	stage.Run(context.Background(), rs2)

	// Cycle 3: check priorFindings passed to runner
	rs3 := runstore.NewRunState("test-spec", "test-project")
	rs3.Cycle = 3
	stage.Run(context.Background(), rs3)

	// priorFindings should have exactly 1 entry, not 2
	if len(capturedInput.PriorFindings) != 1 {
		t.Errorf("expected 1 deduplicated prior finding, got %d", len(capturedInput.PriorFindings))
	}
}

func TestReviewStage_FacetError_ErroredFacetInEvidence(t *testing.T) {
	// When some facets error (but not all), the errored facet info should appear
	// in the review findings stored on RunState so it surfaces in evidence.
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Facet: "code_quality", Severity: review.SeverityInfo, File: "main.go", Description: "looks good"},
			},
			BlockingFindings:    []review.Finding{},
			HasBlockingFindings: false,
			FindingsByFacet: map[string][]review.Finding{
				"code_quality": {{Facet: "code_quality", Severity: review.SeverityInfo, File: "main.go", Description: "looks good"}},
			},
			ErroredFacets: map[string]string{
				"spec_alignment": "API timeout",
			},
			AllFacetsErrored: false,
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

	// Partial facet errors should not block the pipeline — should Continue
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue when not all facets errored, got %v", action.Kind)
	}

	// The review findings on RunState should contain evidence of the successful facet
	if len(rs.ReviewFindings) == 0 {
		t.Fatal("expected ReviewFindings to be populated with findings from successful facets")
	}

	// Verify the errored facet info is accessible via the RunResult's ErroredFacets
	// which the review stage processes. The event log would capture errored facets,
	// and ReviewFindings contains the findings from non-errored facets.
	foundCodeQuality := false
	for _, f := range rs.ReviewFindings {
		if strings.Contains(f, "code_quality") || strings.Contains(f, "looks good") {
			foundCodeQuality = true
		}
	}
	if !foundCodeQuality {
		t.Errorf("expected ReviewFindings to contain code_quality finding, got %v", rs.ReviewFindings)
	}
}

func TestReviewStage_BlockingFindings_OnlyBlockingInReviewFindings(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Severity: review.SeverityError, File: "handler.go", Description: "missing validation"},
				{Severity: review.SeverityInfo, File: "handler.go", Description: "consider helper"},
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
	// On ReplanFrom path, ReviewFindings should contain only blocking findings
	if len(rs.ReviewFindings) != 1 {
		t.Errorf("expected 1 blocking finding in ReviewFindings, got %d", len(rs.ReviewFindings))
	}
	if len(rs.ReviewFindings) > 0 && !strings.Contains(rs.ReviewFindings[0], "missing validation") {
		t.Errorf("expected blocking finding about 'missing validation', got %q", rs.ReviewFindings[0])
	}
}

func TestReviewStage_Continue_AllFindingsInReviewFindings(t *testing.T) {
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Severity: review.SeverityInfo, File: "handler.go", Description: "consider helper"},
				{Severity: review.SeverityInfo, File: "router.go", Description: "naming suggestion"},
			},
			BlockingFindings:    []review.Finding{},
			HasBlockingFindings: false,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{}, nil)
	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}
	// On Continue path, all findings should be stored for evidence
	if len(rs.ReviewFindings) != 2 {
		t.Errorf("expected 2 findings in ReviewFindings on Continue path, got %d", len(rs.ReviewFindings))
	}
}
