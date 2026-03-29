package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/specloop/stages"
)

// TestExecScenarioReviewThrashFirstFixNoThrash verifies that when a blocking
// review finding appears once, a fix task runs, and the next review is clean,
// the run proceeds without a review_thrash_escalated event.
func TestExecScenarioReviewThrashFirstFixNoThrash(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte("# spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	finding := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityError,
		File:        "planner.go",
		Line:        42,
		Description: "buildFixPlanPrompt lacks X",
	}
	failureString := review.ReviewFailuresToStrings([]review.Finding{finding})[0]

	taskRunner := &firstFixNoThrashTaskRunner{}
	provider := &firstFixNoThrashStageProvider{
		storeDir:      tmp,
		taskRunner:    taskRunner,
		failureString: failureString,
		reviewRunner:  &firstFixNoThrashReviewRunner{finding: finding},
	}

	store := runstore.NewStore(tmp)
	run := &execSpecRun{
		specPath:      specPath,
		projectID:     "proj-thrash-no-recur",
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
	runs, err := store.List("proj-thrash-no-recur")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	completed := runs[0]
	if completed.Status != runstore.StatusReadyForReview {
		t.Fatalf("expected run to continue normally to ready_for_review, got %s", completed.Status)
	}

	var thrashRuns []runstore.Task
	for _, seen := range taskRunner.history {
		if seen.TaskID == "task-thrash" {
			thrashRuns = append(thrashRuns, seen)
		}
	}
	if len(thrashRuns) != 2 {
		t.Fatalf("expected fix task to run in two cycles, got %d", len(thrashRuns))
	}
	if thrashRuns[0].ModelTier != "medium" || thrashRuns[1].ModelTier != "medium" {
		t.Fatalf("expected no escalation for fix task, tiers were %q then %q", thrashRuns[0].ModelTier, thrashRuns[1].ModelTier)
	}

	eventsPath := filepath.Join(tmp, "runs", completed.RunID, "events.jsonl")
	raw, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if strings.Contains(string(raw), `"type":"review_thrash_escalated"`) {
		t.Fatalf("did not expect review_thrash_escalated event")
	}
}

type firstFixNoThrashStageProvider struct {
	storeDir      string
	taskRunner    specloop.TaskRunner
	reviewRunner  stages.ReviewRunner
	failureString string
}

func (p *firstFixNoThrashStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget, eventLog *runstore.EventLog) ([]specloop.Stage, error) {
	planStage := &firstFixNoThrashPlanStage{failureString: p.failureString}
	executeStage := stages.NewExecuteStage(p.taskRunner, stages.ExecuteStageConfig{
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
	finalizeStage := &firstFixNoThrashFinalizeStage{}
	return []specloop.Stage{planStage, executeStage, reviewStage, finalizeStage}, nil
}

type firstFixNoThrashPlanStage struct {
	failureString string
}

func (s *firstFixNoThrashPlanStage) Name() string { return "plan" }

func (s *firstFixNoThrashPlanStage) Run(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	rs.Tasks = []runstore.Task{
		{
			TaskID:            "task-thrash",
			Objective:         "fix planner finding",
			Status:            "pending",
			ModelTier:         "medium",
			FailuresAddressed: []string{s.failureString},
			Cycle:             rs.Cycle,
		},
	}
	for i := range rs.Tasks {
		rs.Tasks[i].NormalizeNilFields()
	}
	return specloop.NextAction{Kind: specloop.Continue}, nil
}

type firstFixNoThrashFinalizeStage struct{}

func (s *firstFixNoThrashFinalizeStage) Name() string { return "finalize" }

func (s *firstFixNoThrashFinalizeStage) Run(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	rs.Status = runstore.StatusReadyForReview
	rs.FinalReviewPassed = true
	rs.EndedAt = time.Now()
	return specloop.NextAction{Kind: specloop.Continue}, nil
}

type firstFixNoThrashTaskRunner struct {
	history []runstore.Task
}

func (r *firstFixNoThrashTaskRunner) RunTask(_ context.Context, task runstore.Task) (specloop.TaskResult, error) {
	r.history = append(r.history, task)
	return specloop.TaskResult{TaskID: task.TaskID, Status: "done", Tier: task.ModelTier}, nil
}

func (r *firstFixNoThrashTaskRunner) RepairTask(_ context.Context, _ runstore.Task, _ []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

// firstFixNoThrashReviewRunner emits one blocking finding on cycle 1, then no
// blocking findings starting on cycle 2.
type firstFixNoThrashReviewRunner struct {
	finding review.Finding
}

func (r *firstFixNoThrashReviewRunner) Run(_ context.Context, input review.RunInput) (*review.RunResult, error) {
	result := &review.RunResult{
		FindingsByFacet: map[string][]review.Finding{},
		ErroredFacets:   map[string]string{},
	}
	if input.Cycle == 1 {
		rec := r.finding
		rec.Cycle = input.Cycle
		result.AllFindings = []review.Finding{rec}
		result.BlockingFindings = []review.Finding{rec}
		result.FindingsByFacet[rec.Facet] = []review.Finding{rec}
		result.HasBlockingFindings = true
	}
	result.NormalizeNilFields()
	return result, nil
}
