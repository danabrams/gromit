package specmerge_test

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/internal/specgate"
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
		SpecName:      "test-spec",
		Failures:      failures,
		Priority:      "P1",
		AttemptCount:  0,
		RetryCap:      3,
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

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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

	p := specmerge.NewPipeline(client)
	complete, err := p.IsSpecComplete(specName)
	if err != nil {
		t.Fatalf("IsSpecComplete returned error: %v", err)
	}
	if complete {
		t.Fatal("IsSpecComplete returned true despite open bead")
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
		Config: &config.Config{Validation: config.ValidationConfig{Enabled: true, FullCommands: commands}},
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
