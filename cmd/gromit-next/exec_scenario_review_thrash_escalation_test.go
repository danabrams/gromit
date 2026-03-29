package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/specloop/stages"
)

// TestExecScenarioReviewThrashEscalation verifies that a finding that blocks twice
// in a row emits a review_thrash_escalated event and causes the next replan to
// escalate the corresponding task to the high tier.
func TestExecScenarioReviewThrashEscalation(t *testing.T) {
	tmp := t.TempDir()

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte("# spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	thrashFinding := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityError,
		File:        "thrash.go",
		Line:        42,
		Description: "thrash failure",
	}
	failureString := review.ReviewFailuresToStrings([]review.Finding{thrashFinding})[0]

	taskRunner := &thrashScenarioTaskRunner{}
	provider := &reviewThrashScenarioStageProvider{
		storeDir:      tmp,
		taskRunner:    taskRunner,
		failureString: failureString,
		reviewRunner:  &scenarioReviewRunner{thrashFinding: thrashFinding},
	}

	store := runstore.NewStore(tmp)
	run := &execSpecRun{
		specPath:      specPath,
		projectID:     "proj-thrash",
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}
	if err := run.run(context.Background()); err != nil {
		t.Fatalf("run spec: %v", err)
	}

	runs, err := store.List("proj-thrash")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	eventsPath := filepath.Join(tmp, "runs", runs[0].RunID, "events.jsonl")
	events, err := runstore.NewEventLog(eventsPath).ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var thrashEvent *runstore.ReviewThrashEscalatedEvent
	for _, ev := range events {
		if e, ok := ev.(*runstore.ReviewThrashEscalatedEvent); ok {
			thrashEvent = e
			break
		}
	}
	if thrashEvent == nil {
		t.Fatalf("expected review_thrash_escalated event")
	}
	if thrashEvent.FindingFile != thrashFinding.File {
		t.Errorf("unexpected file: %q", thrashEvent.FindingFile)
	}
	if thrashEvent.FindingDescription != thrashFinding.Description {
		t.Errorf("unexpected description: %q", thrashEvent.FindingDescription)
	}
	if thrashEvent.ConsecutiveCount != 2 {
		t.Errorf("unexpected consecutive count: %d", thrashEvent.ConsecutiveCount)
	}

	var thrashRuns []runstore.Task
	for _, seen := range taskRunner.history {
		if seen.TaskID == "task-thrash" {
			thrashRuns = append(thrashRuns, seen)
		}
	}
	if len(thrashRuns) < 3 {
		t.Fatalf("expected thrash task to run three times, got %d", len(thrashRuns))
	}
	if thrashRuns[0].ModelTier != "medium" {
		t.Errorf("cycle 1 thrash task tier: %q", thrashRuns[0].ModelTier)
	}
	if thrashRuns[1].ModelTier != "medium" {
		t.Errorf("cycle 2 thrash task tier: %q", thrashRuns[1].ModelTier)
	}
	if thrashRuns[2].ModelTier != "high" {
		t.Errorf("cycle 3 thrash task tier: %q", thrashRuns[2].ModelTier)
	}
}

// reviewThrashScenarioStageProvider wires plan/execute/review stages for the
// thrash scenario.
type reviewThrashScenarioStageProvider struct {
	storeDir        string
	taskRunner      specloop.TaskRunner
	reviewRunner    stages.ReviewRunner
	failureString   string
	thrashCountsLog *[]thrashCountsSnapshot
}

func (p *reviewThrashScenarioStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget, eventLog *runstore.EventLog) ([]specloop.Stage, error) {
	planStage := &thrashScenarioPlanStage{
		failureString:   p.failureString,
		thrashCountsLog: p.thrashCountsLog,
	}
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

	return []specloop.Stage{planStage, executeStage, reviewStage}, nil
}

type thrashCountsSnapshot struct {
	Cycle  int
	Counts map[string]int
}

// thrashScenarioPlanStage resets the task list each cycle.
type thrashScenarioPlanStage struct {
	failureString   string
	thrashCountsLog *[]thrashCountsSnapshot
}

func (s *thrashScenarioPlanStage) Name() string { return "plan" }

func (s *thrashScenarioPlanStage) Run(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	if s.thrashCountsLog != nil {
		snapshot := thrashCountsSnapshot{
			Cycle:  rs.Cycle,
			Counts: cloneThrashCounts(rs.ReviewThrashCounts),
		}
		*s.thrashCountsLog = append(*s.thrashCountsLog, snapshot)
	}
	rs.Tasks = []runstore.Task{
		{
			TaskID:            "task-thrash",
			Objective:         "fix thrash failure",
			Status:            "pending",
			ModelTier:         "medium",
			FailuresAddressed: []string{s.failureString},
			Cycle:             rs.Cycle,
		},
		{
			TaskID:    "task-other",
			Objective: "fix other failure",
			Status:    "pending",
			ModelTier: "medium",
			Cycle:     rs.Cycle,
		},
	}
	for i := range rs.Tasks {
		rs.Tasks[i].NormalizeNilFields()
	}
	return specloop.NextAction{Kind: specloop.Continue}, nil
}

// thrashScenarioTaskRunner records task invocations.
type thrashScenarioTaskRunner struct {
	history []runstore.Task
}

func (r *thrashScenarioTaskRunner) RunTask(_ context.Context, task runstore.Task) (specloop.TaskResult, error) {
	r.history = append(r.history, task)
	return specloop.TaskResult{
		TaskID: task.TaskID,
		Status: "done",
		Tier:   task.ModelTier,
	}, nil
}

func (r *thrashScenarioTaskRunner) RepairTask(_ context.Context, _ runstore.Task, _ []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

// scenarioReviewRunner returns a blocking finding on the first two cycles and
// a clean result afterward.
type scenarioReviewRunner struct {
	thrashFinding review.Finding
}

func (r *scenarioReviewRunner) Run(_ context.Context, input review.RunInput) (*review.RunResult, error) {
	result := &review.RunResult{
		FindingsByFacet: map[string][]review.Finding{},
		ErroredFacets:   map[string]string{},
	}
	if input.Cycle <= 2 {
		rec := r.thrashFinding
		rec.Cycle = input.Cycle
		result.AllFindings = []review.Finding{rec}
		result.BlockingFindings = []review.Finding{rec}
		result.FindingsByFacet[rec.Facet] = []review.Finding{rec}
		result.HasBlockingFindings = true
	}
	result.NormalizeNilFields()
	return result, nil
}
