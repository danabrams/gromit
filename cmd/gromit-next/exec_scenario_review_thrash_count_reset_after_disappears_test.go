package main

import (
	"context"
	"encoding/json"
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

func TestScenario_ReviewThrashCountResetsAfterFindingDisappears(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte("# spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	findingF := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityError,
		File:        "thrash.go",
		Line:        42,
		Description: "finding F",
	}
	findingG := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityError,
		File:        "other.go",
		Line:        9,
		Description: "other blocker",
	}
	failureStringF := review.ReviewFailuresToStrings([]review.Finding{findingF})[0]

	provider := &countResetStageProvider{
		storeDir:      tmp,
		taskRunner:    &countResetTaskRunner{},
		failureString: failureStringF,
		reviewRunner: &countResetReviewRunner{
			findingF: findingF,
			findingG: findingG,
		},
	}

	policy := execpolicy.DefaultPolicy()
	policy.Budgets.MaxSpecCycles = 4

	store := runstore.NewStore(tmp)
	run := &execSpecRun{
		specPath:      specPath,
		projectID:     "proj-thrash-count-reset",
		storeDir:      tmp,
		stageProvider: provider,
		policy:        &policy,
		store:         store,
		out:           io.Discard,
	}

	// Invoke
	if err := run.run(context.Background()); err != nil {
		t.Fatalf("run spec: %v", err)
	}

	runs, err := store.List("proj-thrash-count-reset")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	// Assert
	// Exactly one escalation event should exist (triggered at cycle 2 only).
	eventsPath := filepath.Join(tmp, "runs", runs[0].RunID, "events.jsonl")
	rawEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	eventsText := string(rawEvents)
	if strings.Count(eventsText, `"type":"review_thrash_escalated"`) != 1 {
		t.Fatalf("expected exactly one review_thrash_escalated event, got events:\n%s", eventsText)
	}
	if !strings.Contains(eventsText, `"consecutive_count":2`) {
		t.Fatalf("expected escalation event at consecutive_count=2, got events:\n%s", eventsText)
	}

	// After cycle 4 review (final cycle), F should be tracked with count=1.
	rawRun, err := os.ReadFile(filepath.Join(tmp, "runs", runs[0].RunID, "run.json"))
	if err != nil {
		t.Fatalf("read run.json: %v", err)
	}
	var runPayload map[string]any
	if err := json.Unmarshal(rawRun, &runPayload); err != nil {
		t.Fatalf("unmarshal run.json: %v", err)
	}
	thrashCountsRaw, ok := runPayload["review_thrash_counts"]
	if !ok {
		t.Fatalf("expected review_thrash_counts in run.json")
	}
	thrashCounts, ok := thrashCountsRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected review_thrash_counts object, got %T", thrashCountsRaw)
	}
	fpF := countResetThrashFingerprint(findingF)
	if got := intFromAny(thrashCounts[fpF]); got != 1 {
		t.Fatalf("expected review_thrash_counts[%q]=1 after cycle 4 reappearance, got %d", fpF, got)
	}

	// Normal replan path should continue; run should not be terminally blocked.
	if runs[0].Status == runstore.StatusBlocked {
		t.Fatalf("expected non-blocked status (normal replan path), got %s", runs[0].Status)
	}
}

type countResetStageProvider struct {
	storeDir      string
	taskRunner    specloop.TaskRunner
	reviewRunner  stages.ReviewRunner
	failureString string
}

func (p *countResetStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget, eventLog *runstore.EventLog) ([]specloop.Stage, error) {
	plan := &countResetPlanStage{
		failureString: p.failureString,
	}
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
	return []specloop.Stage{plan, execute, reviewStage}, nil
}

type countResetPlanStage struct {
	failureString string
}

func (s *countResetPlanStage) Name() string { return "plan" }

func (s *countResetPlanStage) Run(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	rs.Tasks = []runstore.Task{
		{
			TaskID:            "task-f",
			Objective:         "fix finding F",
			Status:            "pending",
			ModelTier:         "medium",
			FailuresAddressed: []string{s.failureString},
			Cycle:             rs.Cycle,
		},
		{
			TaskID:    "task-other",
			Objective: "unrelated task",
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

type countResetTaskRunner struct{}

func (r *countResetTaskRunner) RunTask(_ context.Context, task runstore.Task) (specloop.TaskResult, error) {
	return specloop.TaskResult{TaskID: task.TaskID, Status: "done", Tier: task.ModelTier}, nil
}

func (r *countResetTaskRunner) RepairTask(_ context.Context, _ runstore.Task, _ []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

type countResetReviewRunner struct {
	findingF review.Finding
	findingG review.Finding
}

func (r *countResetReviewRunner) Run(_ context.Context, input review.RunInput) (*review.RunResult, error) {
	result := &review.RunResult{
		FindingsByFacet: map[string][]review.Finding{},
		ErroredFacets:   map[string]string{},
	}

	emit := func(f review.Finding) {
		rec := f
		rec.Cycle = input.Cycle
		result.AllFindings = []review.Finding{rec}
		result.BlockingFindings = []review.Finding{rec}
		result.FindingsByFacet[rec.Facet] = []review.Finding{rec}
		result.HasBlockingFindings = true
	}

	switch input.Cycle {
	case 1, 2, 4:
		emit(r.findingF)
	case 3:
		emit(r.findingG)
	default:
		result.HasBlockingFindings = false
	}

	result.NormalizeNilFields()
	return result, nil
}

func countResetThrashFingerprint(f review.Finding) string {
	return f.File + "\x00" + f.Description
}
