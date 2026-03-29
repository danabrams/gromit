package acceptor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

type timeoutCapturingAgent struct {
	seenPrompt  string
	seenTimeout time.Duration
}

func (a *timeoutCapturingAgent) EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error) {
	a.seenPrompt = prompt
	if deadline, ok := ctx.Deadline(); ok {
		a.seenTimeout = time.Until(deadline)
	}
	return CriterionResult{
		Status:    StatusPass,
		Rationale: "criterion passes",
	}, nil
}

func TestScenario_SimpleCriterionWithModerateDiffFinishesWithinScaledTimeout(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := &runstore.RunState{
		RunID:              "run-timeout-simple-001",
		SpecID:             "spec-timeout-scaling",
		ProjectID:          "proj-timeout-scaling",
		Status:             runstore.StatusRunning,
		StartedAt:          time.Now().UTC(),
		ReviewThrashCounts: map[string]int{"finding-1": 1},
		Tasks:              []runstore.Task{{TaskID: "task-1", Status: "done"}},
	}
	mustSaveRunState(t, store, rs)

	loaded, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}

	criterion := "RunState.ReviewThrashCounts field exists"
	diffSummary := strings.Repeat("a", 500_000)

	agent := &timeoutCapturingAgent{}
	evaluator := NewEvaluator(agent, DefaultTimeoutConfig())

	// Invoke
	start := time.Now()
	result, err := evaluator.Evaluate(context.Background(), EvaluateInput{
		Criteria:          []string{criterion},
		DiffSummary:       diffSummary,
		TaskResults:       "seeded run has one completed task",
		ValidationResults: "all checks passed",
		ReviewFindings:    strings.Join(loaded.ReviewFindings, "\n"),
	})
	elapsed := time.Since(start)

	// Assert
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	expectedTimeout := 160 * time.Second
	if agent.seenTimeout == 0 {
		t.Fatal("expected per-criterion timeout context, got no deadline")
	}
	if delta := agent.seenTimeout - expectedTimeout; delta < -2*time.Second || delta > 2*time.Second {
		t.Fatalf("per-criterion timeout = %v, want about %v", agent.seenTimeout, expectedTimeout)
	}

	if elapsed >= expectedTimeout {
		t.Fatalf("evaluation exceeded expected per-criterion deadline: elapsed=%v expected<%v", elapsed, expectedTimeout)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 criterion result, got %d", len(result.Results))
	}
	if !strings.Contains(result.Results[0].Status, StatusPass) {
		t.Fatalf("expected status containing %q, got %q", StatusPass, result.Results[0].Status)
	}
	if !strings.Contains(agent.seenPrompt, criterion) {
		t.Fatalf("expected prompt to contain criterion %q", criterion)
	}
}

func mustSaveRunState(t *testing.T, store *runstore.Store, rs *runstore.RunState) {
	t.Helper()
	if err := store.Save(rs); err != nil {
		t.Fatalf("store.Save(%s): %v", rs.RunID, err)
	}
}
