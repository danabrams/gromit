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

// oneRepeatScenarioResult holds captured state after running the one-repeat escalation scenario.
type oneRepeatScenarioResult struct {
	tmp                   string
	priorRunID            string
	fingerprint           string
	failureString         string
	provider              *oneRepeatStageProvider
	taskRunner            *oneRepeatTaskRunner
	persistedThrashCounts map[string]int
	executeCycle3Context  *runstore.ReplanContext
}

// runOneRepeatEscalationScenario seeds, runs, and returns captured state for
// TestScenario_ReviewThrashOneRepeatTriggersEscalation subtests.
func runOneRepeatEscalationScenario(t *testing.T) *oneRepeatScenarioResult {
	t.Helper()

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

	if err := run.run(context.Background()); err != nil {
		t.Fatalf("run spec: %v", err)
	}

	return &oneRepeatScenarioResult{
		tmp:                   tmp,
		priorRunID:            prior.RunID,
		fingerprint:           fingerprint,
		failureString:         failureString,
		provider:              provider,
		taskRunner:            taskRunner,
		persistedThrashCounts: provider.persistedThrashCounts,
		executeCycle3Context:  provider.executeCycle3Context,
	}
}

func TestScenario_ReviewThrashOneRepeatTriggersEscalation(t *testing.T) {
	// Run the shared scenario once and pass results to each focused subtest.
	res := runOneRepeatEscalationScenario(t)

	// Subtest 1: ReviewThrashCounts survives resume and reaches 2 after cycle 2 review.
	t.Run("state_persistence", func(t *testing.T) {
		// Assert via cycle 3 plan-stage snapshot: the count carried into cycle 3 must be 2.
		cycle3ThrashCounts, ok := res.provider.planThrashCounts[3]
		if !ok {
			t.Fatalf("missing thrash counts captured at cycle 3 plan stage")
		}
		if got := cycle3ThrashCounts[res.fingerprint]; got != 2 {
			t.Fatalf("expected ReviewThrashCounts[%q]=2 at cycle 3 plan (persisted from cycle 2 review), got %d", res.fingerprint, got)
		}

		// Assert via persisted run.json: store.Save must have written count=2.
		if res.persistedThrashCounts == nil {
			t.Fatalf("expected persisted review_thrash_counts snapshot after cycle 3 plan stage")
		}
		if got := res.persistedThrashCounts[res.fingerprint]; got != 2 {
			t.Fatalf("expected persisted review_thrash_counts[%q]=2, got %d", res.fingerprint, got)
		}
	})

	// Subtest 2: review_thrash_escalated event emitted with correct fields.
	t.Run("event_emission", func(t *testing.T) {
		eventsPath := filepath.Join(res.tmp, "runs", res.priorRunID, "events.jsonl")
		rawEvents, err := os.ReadFile(eventsPath)
		if err != nil {
			t.Fatalf("read events: %v", err)
		}
		eventsText := string(rawEvents)
		if !strings.Contains(eventsText, `"type":"review_thrash_escalated"`) {
			t.Fatalf("expected review_thrash_escalated event in events.jsonl")
		}
		if !strings.Contains(eventsText, `"consecutive_count":2`) {
			t.Fatalf("expected review_thrash_escalated with consecutive_count=2")
		}

		// The cycle 2 review action must have set EscalatedFailures so downstream stages
		// can drive targeted escalation.
		reviewCycle2, ok := res.provider.reviewActions[2]
		if !ok {
			t.Fatalf("missing captured review action for cycle 2")
		}
		if reviewCycle2.Kind != specloop.ReplanFrom {
			t.Fatalf("expected cycle 2 review action Kind=ReplanFrom, got %v", reviewCycle2.Kind)
		}
		if reviewCycle2.Context == nil {
			t.Fatalf("expected non-nil FailureContext on cycle 2 review action")
		}
		if !containsString(reviewCycle2.Context.EscalatedFailures, res.failureString) {
			t.Fatalf("expected FailureContext.EscalatedFailures to include %q, got %v", res.failureString, reviewCycle2.Context.EscalatedFailures)
		}
	})

	// Subtest 3: only the thrashing task runs at elevated tier; unrelated task stays medium.
	t.Run("targeted_tier_escalation", func(t *testing.T) {
		var cycle3Thrash, cycle3Other *runstore.Task
		for i := range res.taskRunner.history {
			task := res.taskRunner.history[i]
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
		if cycle3Thrash.ModelTier != "high" {
			t.Fatalf("expected task-thrash ModelTier=high (targeted thrash escalation), got %q", cycle3Thrash.ModelTier)
		}
		if cycle3Other.ModelTier != "medium" {
			t.Fatalf("expected task-other ModelTier=medium (no escalation on non-intersecting task), got %q", cycle3Other.ModelTier)
		}

		// Contract: ReplanContext.EscalatedFailures was propagated to the execute stage.
		if res.executeCycle3Context == nil {
			t.Fatalf("expected ReplanContext to be captured on cycle 3 execute stage")
		}
		if !containsString(res.executeCycle3Context.EscalatedFailures, res.failureString) {
			t.Fatalf("expected cycle 3 ReplanContext.EscalatedFailures to include %q, got %v",
				res.failureString, res.executeCycle3Context.EscalatedFailures)
		}

		// task-thrash intersects escalated failures; task-other does not.
		if !containsString(cycle3Thrash.FailuresAddressed, res.failureString) {
			t.Fatalf("expected task-thrash.FailuresAddressed to include escalated failure %q", res.failureString)
		}
		if containsString(cycle3Other.FailuresAddressed, res.failureString) {
			t.Fatalf("expected task-other.FailuresAddressed to NOT intersect with escalated failure %q", res.failureString)
		}
	})
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
	executeCycle3Context  *runstore.ReplanContext // Captured on cycle 3 execute run
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
	// Wrap execute stage to capture ReplanContext on cycle 3
	capturedExecute := &oneRepeatCaptureExecuteStage{
		inner:    execute,
		provider: p,
		cycle3Fn: func(ctx *runstore.ReplanContext) { p.executeCycle3Context = ctx },
	}
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

	return []specloop.Stage{plan, capturedExecute, capturedReview}, nil
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

type oneRepeatCaptureExecuteStage struct {
	inner    specloop.Stage
	provider *oneRepeatStageProvider
	cycle3Fn func(*runstore.ReplanContext)
}

func (s *oneRepeatCaptureExecuteStage) Name() string { return s.inner.Name() }

func (s *oneRepeatCaptureExecuteStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	// Capture ReplanContext on cycle 3 for contract verification
	if rs.Cycle == 3 && rs.ReplanContext != nil && s.cycle3Fn != nil {
		// Clone the context to capture its state before the execute stage modifies run state
		ctxCopy := &runstore.ReplanContext{
			Failures:          append([]string{}, rs.ReplanContext.Failures...),
			EscalatedFailures: append([]string{}, rs.ReplanContext.EscalatedFailures...),
		}
		s.cycle3Fn(ctxCopy)
	}
	return s.inner.Run(ctx, rs)
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
