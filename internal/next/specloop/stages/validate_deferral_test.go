package stages

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/validator"
)

// RED: Test deferral of file_exists and file_contains failures covered by pending tasks
func TestDeferContractFailures_DefersCoveredFile(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "scenario-1",
			AssertionType: "file_exists",
			Details:       "file does not exist",
			Assertion: contract.ContractAssertion{
				FileExists: "src/math/add.go",
			},
		},
	}

	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"src/math/add.go", "src/math/sub.go"},
		},
	}

	result := deferContractFailures(failures, tasks)

	if len(result.remaining) != 0 {
		t.Fatalf("expected 0 remaining failures, got %d", len(result.remaining))
	}
	if len(result.deferred) != 1 {
		t.Fatalf("expected 1 deferred failure, got %d", len(result.deferred))
	}
	if result.deferred[0].ScenarioName != "scenario-1" {
		t.Fatalf("expected deferred failure to be scenario-1, got %s", result.deferred[0].ScenarioName)
	}
}

// RED: Test that uncovered files remain
func TestDeferContractFailures_KeepsUncoveredFile(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "scenario-1",
			AssertionType: "file_exists",
			Details:       "file does not exist",
			Assertion: contract.ContractAssertion{
				FileExists: "src/math/divide.go",
			},
		},
	}

	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"src/math/add.go", "src/math/sub.go"},
		},
	}

	result := deferContractFailures(failures, tasks)

	if len(result.remaining) != 1 {
		t.Fatalf("expected 1 remaining failure, got %d", len(result.remaining))
	}
	if len(result.deferred) != 0 {
		t.Fatalf("expected 0 deferred failures, got %d", len(result.deferred))
	}
}

// RED: Test file_contains assertion deferral
func TestDeferContractFailures_DefersCoveredFileContains(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "scenario-2",
			AssertionType: "file_contains",
			Details:       "pattern not found",
			Assertion: contract.ContractAssertion{
				FileContains: &contract.FileContainsAssertion{
					Path:    "src/math/add.go",
					Pattern: "func Add",
				},
			},
		},
	}

	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"src/math/add.go"},
		},
	}

	result := deferContractFailures(failures, tasks)

	if len(result.remaining) != 0 {
		t.Fatalf("expected 0 remaining failures, got %d", len(result.remaining))
	}
	if len(result.deferred) != 1 {
		t.Fatalf("expected 1 deferred failure, got %d", len(result.deferred))
	}
}

// RED: Test that non-filesystem failures remain
func TestDeferContractFailures_KeepsNonFilesystemFailures(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "scenario-1",
			AssertionType: "file_not_modified",
			Details:       "file was modified",
			Assertion: contract.ContractAssertion{
				FileNotModified: "src/math/add.go",
			},
		},
	}

	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"src/math/add.go"},
		},
	}

	result := deferContractFailures(failures, tasks)

	if len(result.remaining) != 1 {
		t.Fatalf("expected 1 remaining failure, got %d", len(result.remaining))
	}
	if len(result.deferred) != 0 {
		t.Fatalf("expected 0 deferred failures, got %d", len(result.deferred))
	}
}

// RED: Test that first task in slice order wins
func TestDeferContractFailures_FirstTaskWins(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "scenario-1",
			AssertionType: "file_exists",
			Assertion: contract.ContractAssertion{
				FileExists: "src/math/add.go",
			},
		},
	}

	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"src/math/add.go"},
		},
		{
			TaskID:              "task-2",
			Status:              "pending",
			ExpectedTouchedArea: []string{"src/math/add.go"},
		},
	}

	result := deferContractFailures(failures, tasks)

	if len(result.deferred) != 1 {
		t.Fatalf("expected 1 deferred failure, got %d", len(result.deferred))
	}
}

// RED: Test exact string matching
func TestDeferContractFailures_ExactStringMatching(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "scenario-1",
			AssertionType: "file_exists",
			Assertion: contract.ContractAssertion{
				FileExists: "src/math/add.go",
			},
		},
	}

	tasks := []runstore.Task{
		{
			TaskID:              "task-1",
			Status:              "pending",
			ExpectedTouchedArea: []string{"src/math/add"}, // Different path
		},
	}

	result := deferContractFailures(failures, tasks)

	if len(result.remaining) != 1 {
		t.Fatalf("expected 1 remaining failure (no match), got %d", len(result.remaining))
	}
	if len(result.deferred) != 0 {
		t.Fatalf("expected 0 deferred failures, got %d", len(result.deferred))
	}
}

// RED: Test empty tasks slice
func TestDeferContractFailures_EmptyTasks(t *testing.T) {
	failures := []contract.ContractFailure{
		{
			ScenarioName:  "scenario-1",
			AssertionType: "file_exists",
			Assertion: contract.ContractAssertion{
				FileExists: "src/math/add.go",
			},
		},
	}

	result := deferContractFailures(failures, []runstore.Task{})

	if len(result.remaining) != 1 {
		t.Fatalf("expected 1 remaining failure, got %d", len(result.remaining))
	}
	if len(result.deferred) != 0 {
		t.Fatalf("expected 0 deferred failures, got %d", len(result.deferred))
	}
}

// --- Integration Tests ---

// RED: Test Run() method defers failures and emits events
func TestValidateStage_Run_DeferralIntegration(t *testing.T) {
	// Create temporary directories
	tmpEventDir := t.TempDir()
	tmpEvidenceDir := t.TempDir()
	tmpWorkDir := t.TempDir()
	tmpRepoDir := t.TempDir()

	// Create temporary event log file
	tmpEventFile := tmpEventDir + "/events.jsonl"

	// Create contract file in the evidence directory
	contractPath := tmpEvidenceDir + "/scenario-contracts.yaml"
	os.WriteFile(contractPath, []byte(`scenarios:
- name: scenario-1
  assertions:
  - file_exists: "src/math/add.go"
  - file_exists: "src/utils/helper.go"
`), 0o644)

	// Setup fake validator (passes)
	mockVal := &fakeValidator{
		result: validator.FinalResult{Pass: true},
	}

	// Setup fake contract evaluator with mixed failures
	mockEval := &fakeContractEvaluator{
		failures: []contract.ContractFailure{
			{
				ScenarioName:  "scenario-1",
				AssertionType: "file_exists",
				Details:       `file "src/math/add.go" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "src/math/add.go", // This will be deferred (task-1 covers it)
				},
			},
			{
				ScenarioName:  "scenario-1",
				AssertionType: "file_exists",
				Details:       `file "src/utils/helper.go" does not exist`,
				Assertion: contract.ContractAssertion{
					FileExists: "src/utils/helper.go", // This will NOT be deferred (no task covers it)
				},
			},
		},
	}

	// Setup event log
	eventLog := runstore.NewEventLog(tmpEventFile)

	// Setup validate stage config
	cfg := ValidateStageConfig{
		WorkDir:     tmpWorkDir,
		EvidenceDir: tmpEvidenceDir,
		RepoDir:     tmpRepoDir,
	}

	stage := NewValidateStage(mockVal, cfg, eventLog, mockEval, nil)

	// Create RunState with a pending task that covers src/math/add.go
	rs := &runstore.RunState{
		RunID:  "run-123",
		SpecID: "spec-456",
		Cycle:  1,
		Tasks: []runstore.Task{
			{
				TaskID:              "task-1",
				Status:              "pending",
				ExpectedTouchedArea: []string{"src/math/add.go"},
			},
		},
	}

	// Run the validate stage
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify: should replan due to non-deferred failure
	// ReplanFrom is NextActionKind(1)
	if action.Kind != 1 {
		t.Fatalf("expected ReplanFrom action (kind=1), got kind=%v", action.Kind)
	}

	// Verify: action context should contain only non-deferred failures
	if action.Context == nil || len(action.Context.Failures) == 0 {
		t.Fatalf("expected failures in action context")
	}

	failures := action.Context.Failures
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure (non-deferred), got %d: %v", len(failures), failures)
	}

	// The remaining failure should be about src/utils/helper.go
	if !strings.Contains(failures[0], "src/utils/helper.go") {
		t.Fatalf("expected failure about src/utils/helper.go, got: %s", failures[0])
	}

	// Verify: ContractDeferredEvent was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("failed to read event log: %v", err)
	}

	deferredEvents := 0
	for _, ev := range events {
		if ev.EventType() == "contract_deferred" {
			deferredEvents++
			// Verify event structure
			data, _ := json.Marshal(ev)
			var deferrredEv runstore.ContractDeferredEvent
			json.Unmarshal(data, &deferrredEv)

			if deferrredEv.ScenarioName != "scenario-1" {
				t.Fatalf("expected scenario-1, got %s", deferrredEv.ScenarioName)
			}
			if deferrredEv.FilePath != "src/math/add.go" {
				t.Fatalf("expected file path src/math/add.go, got %s", deferrredEv.FilePath)
			}
			if deferrredEv.TaskID != "task-1" {
				t.Fatalf("expected task-1, got %s", deferrredEv.TaskID)
			}
		}
	}

	if deferredEvents != 1 {
		t.Fatalf("expected 1 ContractDeferredEvent, got %d", deferredEvents)
	}
}
