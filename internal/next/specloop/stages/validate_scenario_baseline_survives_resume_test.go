package stages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

func TestScenario_BaselineSurvivesResume(t *testing.T) {
	// Seed
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)
	workDir := t.TempDir()
	eventLogPath := filepath.Join(storeDir, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	prior := runstore.NewRunState("spec-001", "proj-001")
	prior.Status = runstore.StatusNeedsHuman
	prior.EndedAt = time.Now()
	prior.Cycle = 1
	prior.BaselineFailures = map[string]string{"unit-tests": "baseline fail"}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	resumed, err := store.Get(prior.RunID)
	if err != nil {
		t.Fatalf("load run for resume: %v", err)
	}
	resumed.Resumed = true
	resumed.Status = runstore.StatusRunning
	resumed.EndedAt = time.Time{}
	resumed.Cycle++

	if got := resumed.BaselineFailures["unit-tests"]; got != "baseline fail" {
		t.Fatalf("baseline failures not restored from persisted state: got %q", got)
	}

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "baseline fail"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: workDir}, eventLog, nil, nil)

	// Invoke
	action1, err := stage.Run(context.Background(), resumed)
	if err != nil {
		t.Fatalf("first validate run: %v", err)
	}
	if err := store.Save(resumed); err != nil {
		t.Fatalf("save resumed run after first validate: %v", err)
	}

	reloaded, err := store.Get(prior.RunID)
	if err != nil {
		t.Fatalf("reload resumed run: %v", err)
	}
	reloaded.Cycle++
	action2, err := stage.Run(context.Background(), reloaded)
	if err != nil {
		t.Fatalf("second validate run: %v", err)
	}

	// Assert
	if action1.Kind != specloop.Continue {
		t.Fatalf("first validate action = %v, want Continue", action1.Kind)
	}
	if action2.Kind != specloop.Continue {
		t.Fatalf("second validate action = %v, want Continue", action2.Kind)
	}
	if !reloaded.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true when all failures are baseline-excluded after resumed validate cycles")
	}
	if got := reloaded.BaselineFailures["unit-tests"]; got != "baseline fail" {
		t.Fatalf("baseline failure output changed across resumed cycles: got %q", got)
	}

	eventBytes, err := os.ReadFile(eventLogPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	eventsText := string(eventBytes)
	if !strings.Contains(eventsText, "baseline_failure_excluded") {
		t.Fatalf("expected baseline exclusion events in log, got: %s", eventsText)
	}
	if !strings.Contains(eventsText, "unit-tests") {
		t.Fatalf("expected excluded check name in log, got: %s", eventsText)
	}
	if got := strings.Count(eventsText, "baseline_failure_excluded"); got < 2 {
		t.Fatalf("expected exclusion across both validate cycles, got count=%d", got)
	}
}
