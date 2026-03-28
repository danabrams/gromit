package stages

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

type scenarioAlwaysFailingInspector struct {
	failures map[string][]string
}

func (s *scenarioAlwaysFailingInspector) Inspect(_ context.Context, task runstore.Task) specloop.InspectResult {
	return specloop.InspectResult{
		Pass:     false,
		Failures: append([]string(nil), s.failures[task.TaskID]...),
	}
}

func (s *scenarioAlwaysFailingInspector) SetKnownGaps(string) {}

func TestScenario_BuildAlsoFails_NoAnnotation(t *testing.T) {
	// Seed: create a store + run state with one pending task that includes both
	// a build check and a grep proof check.
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.RunID = "run-build-and-grep-fail"
	rs.Tasks = []runstore.Task{{
		TaskID:      "t-001",
		Status:      "pending",
		Objective:   "add title flag",
		ProofChecks: []string{"go build ./...", "grep -q '--title' cmd/foo.go"},
	}}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save runstate: %v", err)
	}

	seeded, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get runstate: %v", err)
	}

	buildFailure := "go build ./...: exit status 1: undefined: missingSymbol"
	grepFailure := "grep -q '--title' cmd/foo.go: exit status 1"

	runner := &fakeTaskRunner{
		results: []specloop.TaskResult{{Status: "done"}},
	}
	inspector := &scenarioAlwaysFailingInspector{
		failures: map[string][]string{
			"t-001": {buildFailure, grepFailure},
		},
	}

	// Invoke 1: run task loop directly to assert TaskResult.Failures contents.
	results, err := specloop.RunTaskLoop(context.Background(), seeded.Tasks, runner, specloop.TaskLoopConfig{
		MaxRetries: 1,
		Inspector:  inspector,
	})
	if err != nil {
		t.Fatalf("RunTaskLoop: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 task result, got %d", len(results))
	}

	// Assert 1: both build and grep failure messages are present in TaskResult.Failures,
	// and none are annotated as suspect-proof-check.
	taskFailures := strings.Join(results[0].Failures, "\n")
	if !strings.Contains(taskFailures, buildFailure) {
		t.Fatalf("expected build failure in TaskResult.Failures, got: %v", results[0].Failures)
	}
	if !strings.Contains(taskFailures, grepFailure) {
		t.Fatalf("expected grep failure in TaskResult.Failures, got: %v", results[0].Failures)
	}
	for _, f := range results[0].Failures {
		if strings.Contains(f, "[suspect-proof-check]") {
			t.Fatalf("did not expect suspect annotation in TaskResult.Failures, got: %v", results[0].Failures)
		}
	}

	// Invoke 2: run ExecuteStage to assert FailureContext.Failures has no annotation.
	runner2 := &fakeTaskRunner{
		results: []specloop.TaskResult{{Status: "done"}},
	}
	stage := NewExecuteStage(runner2, ExecuteStageConfig{
		MaxRetries: 1,
		Inspector:  inspector,
	})
	action, err := stage.Run(context.Background(), seeded)
	if err != nil {
		t.Fatalf("ExecuteStage.Run: %v", err)
	}

	// Assert 2: all tasks failed, so we should replan from FailureContext with both
	// failures present and no suspect-proof-check annotation.
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected non-nil failure context")
	}
	ctxFailures := strings.Join(action.Context.Failures, "\n")
	if !strings.Contains(ctxFailures, buildFailure) {
		t.Fatalf("expected build failure in FailureContext.Failures, got: %v", action.Context.Failures)
	}
	if !strings.Contains(ctxFailures, grepFailure) {
		t.Fatalf("expected grep failure in FailureContext.Failures, got: %v", action.Context.Failures)
	}
	for _, f := range action.Context.Failures {
		if strings.Contains(f, "[suspect-proof-check]") {
			t.Fatalf("did not expect suspect annotation in FailureContext.Failures, got: %v", action.Context.Failures)
		}
	}
}
