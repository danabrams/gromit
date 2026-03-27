package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
)

func TestScenario_PreexistingFailureExcludedFromReplan(t *testing.T) {
	// Seed
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)
	workDir := filepath.Join(storeDir, "repo")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte("module scenario\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "dummy.go"), []byte("package scenario\n\nfunc X() {}\n"), 0o644); err != nil {
		t.Fatalf("write dummy.go: %v", err)
	}

	rs := runstore.NewRunState("spec-preexisting-failure", "proj-001")
	rs.Cycle = 2
	rs.BaselineFailures = map[string]string{
		"unit-tests": "FAIL\t./internal/unrelatedpkg",
	}
	if err := store.Save(rs); err != nil {
		t.Fatalf("save seeded run: %v", err)
	}

	eventLogPath := filepath.Join(store.RunDir(rs.RunID), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	v := &fakeValidator{
		result: validator.FinalResult{
			Pass: false,
			AlwaysRun: validator.CheckResults{
				Results: []validator.CheckResult{
					{Name: "unit-tests", Pass: false, Output: "FAIL\t./internal/unrelatedpkg"},
				},
			},
			ProjectChecks: validator.CheckResults{},
		},
	}

	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: workDir}, eventLog, nil, nil)

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("validate run: %v", err)
	}

	// Assert
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue when only baseline failure blocks, got %v", action.Kind)
	}
	if !rs.FinalValidationPassed {
		t.Fatal("expected FinalValidationPassed to be true when all failures are baseline-excluded")
	}
	if rs.LastFinalValidation == nil || rs.LastFinalValidation.Pass {
		t.Fatalf("expected LastFinalValidation.Pass false, got %+v", rs.LastFinalValidation)
	}

	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("read event log: %v", err)
	}
	found := false
	for _, ev := range events {
		if bfe, ok := ev.(*runstore.BaselineFailureExcludedEvent); ok {
			found = true
			if bfe.CheckName != "unit-tests" {
				t.Fatalf("baseline_failure_excluded check_name = %q, want unit-tests", bfe.CheckName)
			}
			if bfe.RunID != rs.RunID {
				t.Fatalf("baseline_failure_excluded run_id = %q, want %q", bfe.RunID, rs.RunID)
			}
			if bfe.SpecID != rs.SpecID {
				t.Fatalf("baseline_failure_excluded spec_id = %q, want %q", bfe.SpecID, rs.SpecID)
			}
			if bfe.ProjectID != rs.ProjectID {
				t.Fatalf("baseline_failure_excluded project_id = %q, want %q", bfe.ProjectID, rs.ProjectID)
			}
			if bfe.Cycle != rs.Cycle {
				t.Fatalf("baseline_failure_excluded cycle = %d, want %d", bfe.Cycle, rs.Cycle)
			}
		}
	}
	if !found {
		t.Fatal("baseline_failure_excluded event not emitted")
	}
}
