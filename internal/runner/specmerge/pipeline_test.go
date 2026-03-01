package specmerge_test

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/internal/specgate"
	"github.com/danabrams/gromit/internal/tracker"
)

type fakeBeadCreator struct {
	createFn func(ctx context.Context, title, description, priority string, labels []string) (string, error)
}

var _ specgate.BeadCreator = (*fakeBeadCreator)(nil)

func (f *fakeBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	if f.createFn == nil {
		return "", nil
	}
	return f.createFn(ctx, title, description, priority, labels)
}

func TestHandleStageFailure_CreateFixBeads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	failures := []specgate.CriterionResult{
		{Criterion: "Test quality", Passed: false, Evidence: "missing tests"},
	}

	var createdBeadIDs []string
	creator := &fakeBeadCreator{
		createFn: func(_ context.Context, title, description, priority string, labels []string) (string, error) {
			createdBeadIDs = append(createdBeadIDs, "bead-1")
			return "bead-1", nil
		},
	}

	deps := specmerge.FixBeadDependencies{
		BeadCreator: creator,
	}

	opts := specmerge.HandleStageFailureOptions{
		SpecName:     "test-spec",
		Failures:     failures,
		Priority:     "P1",
		AttemptCount: 0,
		RetryCap:     3,
	}

	err := specmerge.HandleStageFailure(ctx, deps, opts)
	if err != nil {
		t.Fatalf("HandleStageFailure returned error: %v", err)
	}

	if len(createdBeadIDs) != 1 {
		t.Fatalf("HandleStageFailure created %d beads, want 1", len(createdBeadIDs))
	}
}

func TestCheckRetryCapExceeded_AtCapOrBeyond(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		attemptCount int
		retryCap     int
		want         bool
	}{
		{
			name:         "attempt equals cap",
			attemptCount: 3,
			retryCap:     3,
			want:         true,
		},
		{
			name:         "attempt exceeds cap",
			attemptCount: 4,
			retryCap:     3,
			want:         true,
		},
		{
			name:         "attempt below cap",
			attemptCount: 2,
			retryCap:     3,
			want:         false,
		},
		{
			name:         "zero attempts at zero cap",
			attemptCount: 0,
			retryCap:     0,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := specmerge.CheckRetryCapExceeded(tt.attemptCount, tt.retryCap)
			if err != nil {
				t.Fatalf("CheckRetryCapExceeded returned error: %v", err)
			}
			if result != tt.want {
				t.Fatalf("CheckRetryCapExceeded(%d, %d) = %v, want %v", tt.attemptCount, tt.retryCap, result, tt.want)
			}
		})
	}
}

func TestEmitRetryCapReachedAlert_ReturnsAlert(t *testing.T) {
	t.Parallel()

	specName := "test-spec"
	retryCap := 3

	alert := specmerge.EmitRetryCapReachedAlert(specName, retryCap)

	if alert == "" {
		t.Fatalf("EmitRetryCapReachedAlert returned empty string, want non-empty alert message")
	}

	if !contains(alert, specName) {
		t.Fatalf("alert = %q, want to contain spec name %q", alert, specName)
	}

	if !contains(alert, "3") {
		t.Fatalf("alert = %q, want to contain retry cap 3", alert)
	}
}

func TestPipeline_Trigger_CreatesFixBeadsOnStageFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	failure := specmerge.StageResult{
		StageName: "spec_conformance",
		Passed:    false,
		ReviewResult: &review.ReviewResult{
			Passed:  false,
			Summary: "spec needs tightening",
		},
	}

	var created []string
	creator := &fakeBeadCreator{
		createFn: func(_ context.Context, title, description, priority string, labels []string) (string, error) {
			created = append(created, title)
			return "bead-1", nil
		},
	}

	flow := &fakeFlowExecutor{
		runFn: func(_ context.Context, specName string) (*specmerge.FlowResult, error) {
			if specName != "payments" {
				t.Fatalf("specName = %q, want payments", specName)
			}
			return &specmerge.FlowResult{
				StageResults: []specmerge.StageResult{failure},
			}, specmerge.StageFailureError{Result: failure}
		},
	}

	p := specmerge.NewPipeline(nil, nil, flow, specmerge.FixBeadDependencies{BeadCreator: creator}, 3)
	if err := p.Trigger(ctx, "payments"); err == nil {
		t.Fatal("expected pipeline to return an error for stage failure")
	}
	if len(created) != 1 {
		t.Fatalf("created %d fix beads, want 1", len(created))
	}
}

func TestPipeline_Trigger_RetryCapReachedAlerts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	failure := specmerge.StageResult{
		StageName: "code_quality",
		Passed:    false,
		ReviewResult: &review.ReviewResult{
			Passed:  false,
			Summary: "code quality violations",
		},
	}

	flow := &fakeFlowExecutor{
		runFn: func(_ context.Context, specName string) (*specmerge.FlowResult, error) {
			return &specmerge.FlowResult{
				StageResults: []specmerge.StageResult{failure},
			}, specmerge.StageFailureError{Result: failure}
		},
	}

	p := specmerge.NewPipeline(nil, nil, flow, specmerge.FixBeadDependencies{BeadCreator: &fakeBeadCreator{}}, 1)
	err := p.Trigger(ctx, "payments")
	if err == nil {
		t.Fatal("expected retry-cap alert error")
	}
	alert := specmerge.EmitRetryCapReachedAlert("payments", 1)
	if !strings.Contains(err.Error(), alert) {
		t.Fatalf("error = %q, want to contain alert %q", err.Error(), alert)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type fakeFlowExecutor struct {
	runFn func(ctx context.Context, specName string) (*specmerge.FlowResult, error)
}

func (f *fakeFlowExecutor) Run(ctx context.Context, specName string) (*specmerge.FlowResult, error) {
	if f == nil || f.runFn == nil {
		return nil, nil
	}
	return f.runFn(ctx, specName)
}

func TestPipeline_IsSpecComplete_FalseWithOpenBead(t *testing.T) {
	t.Parallel()

	const specName = "payments"
	client := &fakeBeadQuery{
		listFn: func(label string) ([]*bead.Bead, error) {
			if label != "spec:"+specName {
				t.Fatalf("label = %q, want spec:%s", label, specName)
			}
			return []*bead.Bead{
				{ID: "bead-1", Status: "open"},
				{ID: "bead-2", Status: "closed"},
			}, nil
		},
	}

	p := specmerge.NewPipeline(client, nil)
	complete, err := p.IsSpecComplete(specName)
	if err != nil {
		t.Fatalf("IsSpecComplete returned error: %v", err)
	}
	if complete {
		t.Fatal("IsSpecComplete returned true despite open bead")
	}
}

func TestPipeline_TriggerCapturesCycleRecord(t *testing.T) {
	t.Parallel()

	specName := "payments"
	query := &fakeBeadQuery{listFn: func(_ string) ([]*bead.Bead, error) {
		return nil, nil
	}}

	t.Run("emitter configured", func(t *testing.T) {
		t.Parallel()
		var captured specmerge.CycleRecord
		emitter := &fakeCycleRecordEmitter{
			captureFn: func(_ context.Context, record specmerge.CycleRecord) error {
				captured = record
				return nil
			},
		}
		p := specmerge.NewPipeline(query, emitter)
		if err := p.Trigger(context.Background(), specName); err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}
		if captured.SpecID != specName {
			t.Fatalf("captured spec = %q, want %q", captured.SpecID, specName)
		}
		if captured.CycleEndPresentedAt.IsZero() {
			t.Fatalf("captured presented time zero, want non-zero")
		}
	})

	t.Run("emitter disabled", func(t *testing.T) {
		t.Parallel()
		p := specmerge.NewPipeline(query, nil)
		if err := p.Trigger(context.Background(), specName); err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}
	})
}

func TestTrackerBeadQueryConvertsTrackerItems(t *testing.T) {
	t.Parallel()

	client := &stubTrackerBeadQueryClient{
		listFn: func(label string) ([]tracker.Item, error) {
			if label != "spec:test" {
				t.Fatalf("label = %q, want spec:test", label)
			}
			return []tracker.Item{
				{
					ID:     "b-1",
					Title:  "Bead",
					Status: tracker.StatusOpen,
					Metadata: map[string]string{
						"type":     "task",
						"labels":   `["spec:test"]`,
						"priority": "1",
					},
				},
				{
					ID:     "b-2",
					Title:  "Closed",
					Status: tracker.StatusClosed,
					Metadata: map[string]string{
						"type":     "task",
						"labels":   `["spec:test"]`,
						"priority": "2",
					},
				},
			}, nil
		},
	}

	query := specmerge.NewTrackerBeadQuery(client)
	if query == nil {
		t.Fatal("NewTrackerBeadQuery returned nil")
	}

	beads, err := query.ListWithLabel("spec:test")
	if err != nil {
		t.Fatalf("ListWithLabel returned error: %v", err)
	}
	if len(beads) != 2 {
		t.Fatalf("len(beads) = %d, want 2", len(beads))
	}
	if beads[0].ID != "b-1" {
		t.Fatalf("first bead ID = %s, want b-1", beads[0].ID)
	}
}

type fakeBeadQuery struct {
	listFn func(label string) ([]*bead.Bead, error)
}

func (f *fakeBeadQuery) ListWithLabel(label string) ([]*bead.Bead, error) {
	if f == nil || f.listFn == nil {
		return nil, nil
	}
	return f.listFn(label)
}

type fakeCycleRecordEmitter struct {
	captureFn func(context.Context, specmerge.CycleRecord) error
}

func (f *fakeCycleRecordEmitter) CaptureCycleRecord(ctx context.Context, record specmerge.CycleRecord) error {
	if f == nil || f.captureFn == nil {
		return nil
	}
	return f.captureFn(ctx, record)
}

type stubTrackerBeadQueryClient struct {
	listFn func(label string) ([]tracker.Item, error)
}

func (s *stubTrackerBeadQueryClient) Ready(ctx context.Context) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerBeadQueryClient) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerBeadQueryClient) Show(ctx context.Context, id string) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerBeadQueryClient) Search(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerBeadQueryClient) Create(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerBeadQueryClient) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerBeadQueryClient) Update(ctx context.Context, req tracker.UpdateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerBeadQueryClient) ListWithLabel(ctx context.Context, label string) ([]tracker.Item, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn(label)
}
func (s *stubTrackerBeadQueryClient) Close(ctx context.Context, id string) error {
	return nil
}
func (s *stubTrackerBeadQueryClient) Sync(ctx context.Context) error {
	return nil
}
func (s *stubTrackerBeadQueryClient) AddComment(ctx context.Context, id, comment string) error {
	return nil
}
func (s *stubTrackerBeadQueryClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}

func TestRunStage1Validation_FailsOnValidationCommandError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	commands := []string{"cmd-one", "cmd-two"}
	var seen []string
	runner := func(_ context.Context, command, workDir string) (string, string, int, error) {
		seen = append(seen, command)
		return "", "stderr detail", 1, nil
	}
	deps := specmerge.Stage1ValidationDependencies{
		CmdRunner: runner,
		GetDiff: func(_ context.Context) (string, error) {
			return "diff --git", nil
		},
	}
	res, err := specmerge.RunStage1Validation(ctx, deps, specmerge.Stage1ValidationOptions{
		Config:  &config.Config{Validation: config.ValidationConfig{Enabled: true, FullCommands: commands}},
		WorkDir: "/repo",
	})
	if err != nil {
		t.Fatalf("RunStage1Validation returned error: %v", err)
	}
	if res.Success {
		t.Fatal("expected validation gate to fail, but success flag was true")
	}
	if res.Diff != "diff --git" {
		t.Fatalf("diff = %q, want diff --git", res.Diff)
	}
	if len(seen) != 1 {
		t.Fatalf("run commands %v, want only first", seen)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(res.Failures))
	}
	failure := res.Failures[0]
	if failure.Criterion == "" {
		t.Fatal("expected criterion name to be populated")
	}
	if failure.Passed {
		t.Fatal("criterion should be marked as failed")
	}
	if !strings.Contains(failure.Evidence, "stderr detail") {
		t.Fatalf("evidence = %q, want to include stderr detail", failure.Evidence)
	}
}
