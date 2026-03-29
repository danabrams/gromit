package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/specloop/stages"
)

func TestScenario_EscalationFailsRunBlocked(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte("# spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	stuckFinding := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityError,
		File:        "stuck.go",
		Line:        42,
		Description: "stuck review finding",
	}
	stuckFailure := review.ReviewFailuresToStrings([]review.Finding{stuckFinding})[0]

	taskRunner := &blockedRunTaskRunner{}
	provider := &blockedRunStageProvider{
		storeDir:     tmp,
		taskRunner:   taskRunner,
		reviewRunner: &alwaysBlockingReviewRunner{finding: stuckFinding},
		stuckFailure: stuckFailure,
	}

	store := runstore.NewStore(tmp)
	run := &execSpecRun{
		specPath:      specPath,
		projectID:     "proj-stuck",
		resumeCycles:  5,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}

	// Invoke
	if err := run.run(context.Background()); err != nil {
		t.Fatalf("run spec: %v", err)
	}

	// Assert
	runs, err := store.List("proj-stuck")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Status != runstore.StatusBlocked {
		t.Fatalf("expected blocked status, got %s", runs[0].Status)
	}

	if got := provider.reviewActions[3].Kind; got != specloop.Blocked {
		t.Fatalf("expected cycle-3 review action Blocked, got %v", got)
	}
	if !strings.Contains(runs[0].TerminalReason, "stuck") {
		t.Fatalf("expected terminal reason to reference stuck finding, got %q", runs[0].TerminalReason)
	}

	eventsPath := filepath.Join(tmp, "runs", runs[0].RunID, "events.jsonl")
	rawEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !strings.Contains(string(rawEvents), `"type":"terminal_state"`) {
		t.Fatalf("expected terminal_state event")
	}
	if !strings.Contains(string(rawEvents), `"reason":"stuck`) {
		t.Fatalf("expected terminal_state reason to reference stuck finding, events: %s", string(rawEvents))
	}

	if provider.planCalls != 3 {
		t.Fatalf("expected planning to stop at cycle 3, got %d plan calls", provider.planCalls)
	}
	if len(taskRunner.history) != 3 {
		t.Fatalf("expected exactly 3 executed tasks (one per cycle), got %d", len(taskRunner.history))
	}
}

type blockedRunStageProvider struct {
	storeDir      string
	taskRunner    specloop.TaskRunner
	reviewRunner  stages.ReviewRunner
	stuckFailure  string
	planCalls     int
	reviewActions map[int]specloop.NextAction
}

func (p *blockedRunStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget, eventLog *runstore.EventLog) ([]specloop.Stage, error) {
	if p.reviewActions == nil {
		p.reviewActions = map[int]specloop.NextAction{}
	}

	plan := &blockedRunPlanStage{provider: p, stuckFailure: p.stuckFailure}
	execute := stages.NewExecuteStage(p.taskRunner, stages.ExecuteStageConfig{
		MaxRetries:             policy.Budgets.MaxTaskRetries,
		MaxRedecompositions:    policy.Budgets.MaxRedecompositionPasses,
		WorkDir:                p.storeDir,
		MaxTaskDurationSeconds: policy.Budgets.MaxTaskDurationSeconds,
		Budget:                 budget,
		EventLog:               eventLog,
	})

	evidenceDir := filepath.Join(p.storeDir, "runs", rs.RunID, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return nil, err
	}
	reviewStage := stages.NewReviewStage(p.reviewRunner, stages.ReviewStageConfig{
		SpecContent: "# spec",
		EvidenceDir: evidenceDir,
		BaseBranch:  "main",
		DefaultTier: policy.Models.Evaluator,
		FacetTiers:  map[string]string{"spec_alignment": "high"},
		WorkDir:     p.storeDir,
	}, eventLog)

	capturedReview := &blockedRunCaptureReviewStage{provider: p, inner: reviewStage}
	return []specloop.Stage{plan, execute, capturedReview}, nil
}

type blockedRunPlanStage struct {
	provider     *blockedRunStageProvider
	stuckFailure string
}

func (s *blockedRunPlanStage) Name() string { return "plan" }

func (s *blockedRunPlanStage) Run(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	s.provider.planCalls++
	rs.Tasks = []runstore.Task{{
		TaskID:            "task-stuck",
		Objective:         "fix stuck finding",
		Status:            "pending",
		ModelTier:         "medium",
		FailuresAddressed: []string{s.stuckFailure},
		Cycle:             rs.Cycle,
	}}
	for i := range rs.Tasks {
		rs.Tasks[i].NormalizeNilFields()
	}
	return specloop.NextAction{Kind: specloop.Continue}, nil
}

type blockedRunCaptureReviewStage struct {
	provider *blockedRunStageProvider
	inner    specloop.Stage
}

func (s *blockedRunCaptureReviewStage) Name() string { return s.inner.Name() }

func (s *blockedRunCaptureReviewStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	action, err := s.inner.Run(ctx, rs)
	if err != nil {
		return action, err
	}
	s.provider.reviewActions[rs.Cycle] = action
	return action, nil
}

type blockedRunTaskRunner struct {
	history []runstore.Task
}

func (r *blockedRunTaskRunner) RunTask(_ context.Context, task runstore.Task) (specloop.TaskResult, error) {
	r.history = append(r.history, task)
	return specloop.TaskResult{TaskID: task.TaskID, Status: "done", Tier: task.ModelTier}, nil
}

func (r *blockedRunTaskRunner) RepairTask(_ context.Context, _ runstore.Task, _ []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

type alwaysBlockingReviewRunner struct {
	finding review.Finding
}

func (r *alwaysBlockingReviewRunner) Run(_ context.Context, input review.RunInput) (*review.RunResult, error) {
	result := &review.RunResult{
		FindingsByFacet: map[string][]review.Finding{},
		ErroredFacets:   map[string]string{},
	}
	rec := r.finding
	rec.Cycle = input.Cycle
	result.AllFindings = []review.Finding{rec}
	result.BlockingFindings = []review.Finding{rec}
	result.FindingsByFacet[rec.Facet] = []review.Finding{rec}
	result.HasBlockingFindings = true
	result.NormalizeNilFields()
	return result, nil
}
