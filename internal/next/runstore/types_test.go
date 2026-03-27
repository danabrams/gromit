package runstore

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunState_IsTerminal(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"running", false},
		{"ready_for_review", true},
		{"needs_human", true},
		{"blocked", true},
		{"completed", true},
	}
	for _, tc := range cases {
		rs := RunState{Status: tc.status}
		if rs.IsTerminal() != tc.want {
			t.Errorf("status=%s: want IsTerminal=%v", tc.status, tc.want)
		}
	}
}

func TestNewRunState_GeneratesID(t *testing.T) {
	rs := NewRunState("spec-001", "proj-1")
	if rs.RunID == "" {
		t.Fatal("RunID must not be empty")
	}
	if rs.Status != "running" {
		t.Fatalf("want running, got %s", rs.Status)
	}
}

func TestNewRunState_TasksNotNil(t *testing.T) {
	rs := NewRunState("spec-001", "proj-1")
	if rs.Tasks == nil {
		t.Fatal("Tasks must not be nil")
	}
}

func TestRunState_NormalizeNilFields(t *testing.T) {
	rs := &RunState{}
	rs.NormalizeNilFields()
	if rs.Tasks == nil {
		t.Fatal("NormalizeNilFields must set Tasks to empty slice")
	}
}

func TestRunStateNormalizeNilFields_InitializesBaselineFailures(t *testing.T) {
	rs := &RunState{}
	if rs.BaselineFailures != nil {
		t.Fatal("precondition: BaselineFailures should be nil")
	}
	rs.NormalizeNilFields()
	if rs.BaselineFailures == nil {
		t.Fatal("BaselineFailures must not be nil after normalize")
	}
	if len(rs.BaselineFailures) != 0 {
		t.Fatalf("BaselineFailures should be empty after normalize, got %d entries", len(rs.BaselineFailures))
	}
}

func TestTask_NormalizeNilFields(t *testing.T) {
	tk := &Task{}
	tk.NormalizeNilFields()
	if tk.ExpectedTouchedArea == nil {
		t.Fatal("ExpectedTouchedArea must not be nil after normalize")
	}
	if tk.ProofChecks == nil {
		t.Fatal("ProofChecks must not be nil after normalize")
	}
	if tk.FilesChanged == nil {
		t.Fatal("FilesChanged must not be nil after normalize")
	}
	if tk.FailuresAddressed == nil {
		t.Fatal("FailuresAddressed must not be nil after normalize")
	}
}

func TestRunState_ReviewAndAcceptanceFields(t *testing.T) {
	rs := NewRunState("test-spec", "test-project")
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = false
	rs.ReviewFindings = []string{"[spec_alignment] error: handler.go:42 — missing validation"}
	rs.AcceptanceResults = []string{"acceptance:fail: multi-currency — implement missing behavior"}

	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RunState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.FinalReviewPassed {
		t.Error("FinalReviewPassed should round-trip")
	}
	if got.FinalAcceptancePassed {
		t.Error("FinalAcceptancePassed should round-trip as false")
	}
	if len(got.ReviewFindings) != 1 {
		t.Errorf("ReviewFindings should round-trip, got %d", len(got.ReviewFindings))
	}
	if len(got.AcceptanceResults) != 1 {
		t.Errorf("AcceptanceResults should round-trip, got %d", len(got.AcceptanceResults))
	}
}

func TestRunState_NormalizeNilFields_IncludesNewFields(t *testing.T) {
	rs := &RunState{}
	rs.NormalizeNilFields()
	if rs.Tasks == nil {
		t.Error("Tasks should not be nil after NormalizeNilFields")
	}
	if rs.ReplanContext == nil {
		t.Error("ReplanContext should not be nil after NormalizeNilFields")
	}
	if rs.ReviewFindings == nil {
		t.Error("ReviewFindings should not be nil after NormalizeNilFields")
	}
	if rs.AcceptanceResults == nil {
		t.Error("AcceptanceResults should not be nil after NormalizeNilFields")
	}
	if rs.BaselineFailures == nil {
		t.Error("BaselineFailures should not be nil after NormalizeNilFields")
	}
}

func TestRunState_BaselineFailuresRoundTrip(t *testing.T) {
	rs := NewRunState("spec-001", "proj-1")
	rs.BaselineFailures = map[string]string{"unit-tests": "baseline fail"}

	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got RunState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.BaselineFailures) != 1 {
		t.Fatalf("expected 1 baseline failure, got %d", len(got.BaselineFailures))
	}
	if got.BaselineFailures["unit-tests"] != "baseline fail" {
		t.Fatalf("baseline failure output mismatch: got %q", got.BaselineFailures["unit-tests"])
	}
}

func TestRunState_NormalizeNilFields_ReviewAndAcceptanceSlices(t *testing.T) {
	rs := &RunState{}
	// Verify slices start nil
	if rs.ReviewFindings != nil {
		t.Fatal("precondition: ReviewFindings should be nil before normalize")
	}
	if rs.AcceptanceResults != nil {
		t.Fatal("precondition: AcceptanceResults should be nil before normalize")
	}

	rs.NormalizeNilFields()

	if rs.ReviewFindings == nil {
		t.Error("ReviewFindings should be non-nil empty slice after NormalizeNilFields")
	}
	if len(rs.ReviewFindings) != 0 {
		t.Errorf("ReviewFindings should be empty, got %d", len(rs.ReviewFindings))
	}
	if rs.AcceptanceResults == nil {
		t.Error("AcceptanceResults should be non-nil empty slice after NormalizeNilFields")
	}
	if len(rs.AcceptanceResults) != 0 {
		t.Errorf("AcceptanceResults should be empty, got %d", len(rs.AcceptanceResults))
	}
}

func TestTaskLineageEntry_ChainIDsCanBeSet(t *testing.T) {
	entry := &TaskLineageEntry{
		ChainIDs: []string{"chain-1"},
	}
	if len(entry.ChainIDs) != 1 {
		t.Errorf("TaskLineageEntry.ChainIDs should be settable, got %d items", len(entry.ChainIDs))
	}
}

func TestRunState_TaskLineageMap_Initialized(t *testing.T) {
	rs := &RunState{}
	rs.NormalizeNilFields()
	if rs.TaskLineage == nil {
		t.Error("TaskLineage map should be initialized in NormalizeNilFields")
	}
}

func TestTask_HasFixesField(t *testing.T) {
	tk := &Task{
		TaskID: "task-1",
		Fixes:  "fix-1",
	}
	if tk.Fixes != "fix-1" {
		t.Errorf("Task.Fixes should be set to 'fix-1', got %q", tk.Fixes)
	}
}

// TestTask_HasConsecutiveFailsField and TestTask_HasLastErrorField are removed:
// ConsecutiveFails and LastError are no longer on Task — they are stored exclusively
// in TaskLineage (rs.TaskLineage) which is the authoritative store.

func TestNormalizeNilFields_TaskLineage(t *testing.T) {
	rs := &RunState{}
	rs.NormalizeNilFields()
	if rs.TaskLineage == nil {
		t.Fatal("TaskLineage should be initialized")
	}
}

func TestRunState_NormalizeNilFields_DoesNotPreCreateLineageEntries(t *testing.T) {
	// NormalizeNilFields should NOT proactively create TaskLineage entries for tasks.
	// Lineage entries are created lazily in UpdateTaskLineage only when a task actually fails.
	rs := &RunState{
		Tasks: []Task{
			{TaskID: "task-1"},
			{TaskID: "task-2"},
		},
	}
	rs.NormalizeNilFields()

	if rs.TaskLineage == nil {
		t.Fatal("TaskLineage map should be initialized (as empty map)")
	}
	// Entries should NOT have been pre-created for tasks that haven't failed
	for _, task := range rs.Tasks {
		if _, exists := rs.TaskLineage[task.TaskID]; exists {
			t.Errorf("TaskLineage should NOT have pre-created entry for task %s (lazy creation only)", task.TaskID)
		}
	}
}

func TestRunState_ArchitectureConstraints_JSONMarshal(t *testing.T) {
	rs := NewRunState("spec-001", "proj-1")
	rs.ArchitectureConstraints = []string{"constraint-1", "constraint-2"}

	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Check that architecture_constraints key is present in JSON
	if !strings.Contains(string(data), `"architecture_constraints"`) {
		t.Error("JSON should contain 'architecture_constraints' key when non-empty")
	}

	// Unmarshal back and assert actual values
	rs2 := &RunState{}
	if err := json.Unmarshal(data, rs2); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(rs2.ArchitectureConstraints) != 2 {
		t.Errorf("expected 2 constraints after round-trip, got %d", len(rs2.ArchitectureConstraints))
	}
	if rs2.ArchitectureConstraints[0] != "constraint-1" {
		t.Errorf("expected first constraint to be 'constraint-1', got %q", rs2.ArchitectureConstraints[0])
	}
	if rs2.ArchitectureConstraints[1] != "constraint-2" {
		t.Errorf("expected second constraint to be 'constraint-2', got %q", rs2.ArchitectureConstraints[1])
	}
}

func TestRunState_ArchitectureConstraints_JSONUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"run_id": "run-test",
		"spec_id": "spec-001",
		"project_id": "proj-1",
		"status": "running",
		"cycle": 0,
		"started_at": "2026-03-24T00:00:00Z",
		"tasks": [],
		"accumulated_cost": 0,
		"final_validation_passed": false,
		"final_review_passed": false,
		"final_acceptance_passed": false,
		"contracts_written": false,
		"scenario_tests_written": false,
		"architecture_constraints": ["constraint-a", "constraint-b"]
	}`)

	var rs RunState
	if err := json.Unmarshal(jsonData, &rs); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if len(rs.ArchitectureConstraints) != 2 {
		t.Errorf("expected 2 constraints, got %d", len(rs.ArchitectureConstraints))
	}
	if rs.ArchitectureConstraints[0] != "constraint-a" {
		t.Errorf("expected first constraint to be 'constraint-a', got %q", rs.ArchitectureConstraints[0])
	}
	if rs.ArchitectureConstraints[1] != "constraint-b" {
		t.Errorf("expected second constraint to be 'constraint-b', got %q", rs.ArchitectureConstraints[1])
	}
}

func TestRunState_NormalizeNilFields_ArchitectureConstraints(t *testing.T) {
	rs := &RunState{}
	// Verify precondition
	if rs.ArchitectureConstraints != nil {
		t.Fatal("precondition: ArchitectureConstraints should be nil before normalize")
	}

	rs.NormalizeNilFields()

	if rs.ArchitectureConstraints == nil {
		t.Error("ArchitectureConstraints should be non-nil empty slice after NormalizeNilFields")
	}
	if len(rs.ArchitectureConstraints) != 0 {
		t.Errorf("ArchitectureConstraints should be empty, got %d", len(rs.ArchitectureConstraints))
	}
}
