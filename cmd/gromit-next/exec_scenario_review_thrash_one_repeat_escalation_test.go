package main

import (
	"context"
	"fmt"
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

func TestScenario_ReviewThrashOneRepeatTriggersEscalation(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte("# spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	const reviewThrashCountsField = "review_thrash_counts"

	finding := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityError,
		File:        "planner.go",
		Line:        42,
		Description: "buildFixPlanPrompt lacks X",
	}
	fingerprint := oneRepeatThrashFingerprintForTest(finding)
	failureString := review.ReviewFailuresToStrings([]review.Finding{finding})[0]

	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("spec-thrash-repeat", "proj-thrash-repeat")
	prior.Status = runstore.StatusNeedsHuman
	prior.EndedAt = time.Now()
	prior.Cycle = 1
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	taskRunner := &oneRepeatTaskRunner{}
	provider := &oneRepeatStageProvider{
		storeDir:           tmp,
		taskRunner:         taskRunner,
		targetFailure:      failureString,
		reviewRunner:       &oneRepeatReviewRunner{finding: finding},
		store:              store,
		planThrashCounts:   map[int]map[string]int{},
		reviewActions:      map[int]specloop.NextAction{},
		reviewEscalatedSet: map[int][]string{},
	}

	run := &execSpecRun{
		specPath:      specPath,
		projectID:     "proj-thrash-repeat",
		resumeRunID:   prior.RunID,
		resumeCycles:  3,
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

	// Assert: when cycle 2 review completed, thrash count reached 2.
	// Capture the map after cycle 2 by inspecting the plan stage snapshot at cycle 3.
	cycle3ThrashCounts, ok := provider.planThrashCounts[3]
	if !ok {
		t.Fatalf("missing thrash counts captured at cycle 3 plan stage")
	}
	if got := cycle3ThrashCounts[fingerprint]; got != 2 {
		t.Fatalf("expected thrash count 2 after cycle 2 review, got %d", got)
	}

	if provider.persistedThrashCounts == nil {
		t.Fatalf("expected persisted %s snapshot after cycle 2 review", reviewThrashCountsField)
	}
	if got := provider.persistedThrashCounts[fingerprint]; got != 2 {
		t.Fatalf("expected persisted %s[%q]=2, got %d", reviewThrashCountsField, fingerprint, got)
	}

	// Assert: cycle 2 review returned ReplanFrom with escalated failure in context.
	reviewCycle2, ok := provider.reviewActions[2]
	if !ok {
		t.Fatalf("missing captured review action for cycle 2")
	}
	if reviewCycle2.Kind != specloop.ReplanFrom {
		t.Fatalf("expected cycle 2 review action ReplanFrom, got %v", reviewCycle2.Kind)
	}
	if reviewCycle2.Context == nil {
		t.Fatalf("expected non-nil failure context on cycle 2 review action")
	}
	if !containsString(reviewCycle2.Context.Failures, failureString) {
		t.Fatalf("expected FailureContext.Failures to include %q, got %v", failureString, reviewCycle2.Context.Failures)
	}

	// Assert: review_thrash_escalated event emitted with consecutive_count=2.
	eventsPath := filepath.Join(tmp, "runs", prior.RunID, "events.jsonl")
	rawEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !strings.Contains(string(rawEvents), `"type":"review_thrash_escalated"`) {
		t.Fatalf("expected review_thrash_escalated event")
	}
	if !strings.Contains(string(rawEvents), `"consecutive_count":2`) {
		t.Fatalf("expected review_thrash_escalated consecutive_count=2")
	}

	// Assert: in the replan immediately after escalation (cycle 3), both tasks remain medium.
	// Note: targeted escalation based on persisted ReviewEscalatedFailures has been removed.
	// Task escalation will be re-implemented using transient FailureContext in a future refactoring.
	var cycle3Thrash, cycle3Other *runstore.Task
	for i := range taskRunner.history {
		task := taskRunner.history[i]
		if task.Cycle != 3 {
			continue
		}
		switch task.TaskID {
		case "task-thrash":
			cp := task
			cycle3Thrash = &cp
		case "task-other":
			cp := task
			cycle3Other = &cp
		}
	}
	if cycle3Thrash == nil || cycle3Other == nil {
		t.Fatalf("expected both cycle 3 tasks to run, got thrash=%v other=%v", cycle3Thrash, cycle3Other)
	}
	if cycle3Thrash.ModelTier != "medium" {
		t.Fatalf("expected task ModelTier=medium (escalation via persisted field removed), got %q", cycle3Thrash.ModelTier)
	}
	if cycle3Other.ModelTier != "medium" {
		t.Fatalf("expected unaffected task ModelTier=medium, got %q", cycle3Other.ModelTier)
	}
}

type oneRepeatStageProvider struct {
	storeDir              string
	taskRunner            specloop.TaskRunner
	reviewRunner          stages.ReviewRunner
	store                 *runstore.Store
	targetFailure         string
	planThrashCounts      map[int]map[string]int
	reviewActions         map[int]specloop.NextAction
	reviewEscalatedSet    map[int][]string
	persistedThrashCounts map[string]int
}

func (p *oneRepeatStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.RunState, budget *specloop.Budget, eventLog *runstore.EventLog) ([]specloop.Stage, error) {
	plan := &oneRepeatPlanStage{
		targetFailure:     p.targetFailure,
		countsByCycle:     p.planThrashCounts,
		store:             p.store,
		persistAfterCycle: 3,
		onPersistedCounts: func(counts map[string]int) {
			p.persistedThrashCounts = counts
		},
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
	capturedReview := &oneRepeatCaptureStage{inner: reviewStage, actionsByCycle: p.reviewActions}

	return []specloop.Stage{plan, execute, capturedReview}, nil
}

type oneRepeatPlanStage struct {
	targetFailure     string
	countsByCycle     map[int]map[string]int
	store             *runstore.Store
	persistAfterCycle int
	onPersistedCounts func(map[string]int)
	persisted         bool
}

func (s *oneRepeatPlanStage) Name() string { return "plan" }

func (s *oneRepeatPlanStage) Run(_ context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	if s.countsByCycle != nil {
		s.countsByCycle[rs.Cycle] = cloneThrashCounts(rs.ReviewThrashCounts)
	}
	if s.store != nil && !s.persisted && s.persistAfterCycle > 0 && rs.Cycle == s.persistAfterCycle {
		if err := s.store.Save(rs); err != nil {
			return specloop.NextAction{}, fmt.Errorf("persist thrash counts: %w", err)
		}
		s.persisted = true
		if s.onPersistedCounts != nil {
			saved, err := s.store.Get(rs.RunID)
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("load persisted run: %w", err)
			}
			s.onPersistedCounts(cloneThrashCounts(saved.ReviewThrashCounts))
		}
	}

	rs.Tasks = []runstore.Task{
		{
			TaskID:            "task-thrash",
			Objective:         "fix planner finding",
			Status:            "pending",
			ModelTier:         "medium",
			FailuresAddressed: []string{s.targetFailure},
			Cycle:             rs.Cycle,
		},
		{
			TaskID:            "task-other",
			Objective:         "unrelated cleanup",
			Status:            "pending",
			ModelTier:         "medium",
			FailuresAddressed: []string{"review:spec_alignment:error:other.go:unrelated failure"},
			Cycle:             rs.Cycle,
		},
	}
	for i := range rs.Tasks {
		rs.Tasks[i].NormalizeNilFields()
	}
	return specloop.NextAction{Kind: specloop.Continue}, nil
}

type oneRepeatCaptureStage struct {
	inner          specloop.Stage
	actionsByCycle map[int]specloop.NextAction
}

func (s *oneRepeatCaptureStage) Name() string { return s.inner.Name() }

func (s *oneRepeatCaptureStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	action, err := s.inner.Run(ctx, rs)
	if err == nil {
		s.actionsByCycle[rs.Cycle] = action
	}
	return action, err
}

type oneRepeatTaskRunner struct {
	history []runstore.Task
}

func (r *oneRepeatTaskRunner) RunTask(_ context.Context, task runstore.Task) (specloop.TaskResult, error) {
	r.history = append(r.history, task)
	return specloop.TaskResult{TaskID: task.TaskID, Status: "done", Tier: task.ModelTier}, nil
}

func (r *oneRepeatTaskRunner) RepairTask(_ context.Context, _ runstore.Task, _ []string) (specloop.TaskResult, error) {
	return specloop.TaskResult{Status: "done"}, nil
}

type oneRepeatReviewRunner struct {
	finding review.Finding
}

func (r *oneRepeatReviewRunner) Run(_ context.Context, input review.RunInput) (*review.RunResult, error) {
	result := &review.RunResult{
		FindingsByFacet: map[string][]review.Finding{},
		ErroredFacets:   map[string]string{},
	}
	if input.Cycle <= 2 {
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

func oneRepeatThrashFingerprintForTest(f review.Finding) string {
	return f.File + "\x00" + f.Description
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
