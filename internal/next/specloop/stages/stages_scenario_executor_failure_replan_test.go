package stages

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

func TestScenario_ExecutorIntroducedFailureStillTriggersReplan(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	rs := runstore.NewRunState("spec-executor-failure", "proj-cli")
	rs.Cycle = 1
	rs.BaselineFailures = map[string]string{} // init captured clean baseline (no failures)
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run state: %v", err)
	}

	loaded, err := store.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get run state: %v", err)
	}

	eventLog := runstore.NewEventLog(filepath.Join(tmp, "events.jsonl"))
	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "FAIL\tunit-tests: executor introduced bug"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}
	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: tmp}, eventLog, nil, nil)

	// Invoke
	action, err := stage.Run(context.Background(), loaded)
	if err != nil {
		t.Fatalf("validate run: %v", err)
	}

	// Assert
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}
	if action.Context == nil {
		t.Fatal("expected failure context")
	}

	foundUnitTestsFailure := false
	for _, failure := range action.Context.Failures {
		if strings.Contains(failure, "unit-tests") {
			foundUnitTestsFailure = true
			break
		}
	}
	if !foundUnitTestsFailure {
		t.Fatalf("expected unit-tests failure in replan context, got: %v", action.Context.Failures)
	}

	if _, excluded := loaded.BaselineFailures["unit-tests"]; excluded {
		t.Fatalf("unit-tests should not be baseline-excluded, baseline=%v", loaded.BaselineFailures)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, ev := range events {
		if _, ok := ev.(*runstore.BaselineFailureExcludedEvent); ok {
			t.Fatal("did not expect baseline_failure_excluded event for executor-introduced failure")
		}
	}
}
