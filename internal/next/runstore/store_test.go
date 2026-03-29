package runstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStore_CreateAndGet(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")

	if err := s.Save(rs); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Get(rs.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SpecID != "spec-1" {
		t.Fatalf("want spec-1, got %s", loaded.SpecID)
	}
}

func TestStoreSaveGet_PreservesBaselineFailures(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")
	rs.BaselineFailures["unit-tests"] = "baseline failure"
	if err := s.Save(rs); err != nil {
		t.Fatalf("save run state: %v", err)
	}
	loaded, err := s.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get run state: %v", err)
	}
	if len(loaded.BaselineFailures) != 1 {
		t.Fatalf("expected 1 baseline failure, got %d", len(loaded.BaselineFailures))
	}
	if got := loaded.BaselineFailures["unit-tests"]; got != "baseline failure" {
		t.Fatalf("baseline failure mismatch: got %q", got)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := NewStore(t.TempDir())
	_, err := s.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestStore_List(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.Save(NewRunState("spec-1", "proj-1"))
	s.Save(NewRunState("spec-2", "proj-1"))
	s.Save(NewRunState("spec-3", "proj-2"))

	runs, err := s.List("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("want 2 runs, got %d", len(runs))
	}
}

func TestStore_List_Empty(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	runs, err := s.List("proj-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("want 0 runs, got %d", len(runs))
	}
}

func TestStore_RunDir_Layout(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")
	s.Save(rs)

	runDir := s.RunDir(rs.RunID)
	if _, err := os.Stat(filepath.Join(runDir, "run.json")); err != nil {
		t.Fatal("run.json must exist in run dir")
	}

	taskDir := s.TaskDir(rs.RunID, "t-001")
	if !strings.Contains(taskDir, "tasks/t-001") {
		t.Fatalf("unexpected task dir: %s", taskDir)
	}

	evidenceDir := s.EvidenceDir(rs.RunID, "t-001")
	if !strings.Contains(evidenceDir, "tasks/t-001/evidence") {
		t.Fatalf("unexpected evidence dir: %s", evidenceDir)
	}
}

func TestStore_WriteAndReadTaskArtifact(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-1", "proj-1")
	s.Save(rs)

	err := s.WriteTaskArtifact(rs.RunID, "t-001", "result.json", map[string]string{"status": "done"})
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	err = s.ReadTaskArtifact(rs.RunID, "t-001", "result.json", &result)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "done" {
		t.Fatalf("want done, got %s", result["status"])
	}
}

func TestStore_ReadTaskArtifact_NotFound(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	var result map[string]string
	err := s.ReadTaskArtifact("run-xxx", "t-001", "missing.json", &result)
	if err == nil {
		t.Fatal("expected error for missing artifact")
	}
}

func TestRunStore_SaveLoad_PriorReviewFindingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	const payload = `{"focus":"review","details":[{"id":"task-1","severity":"warning"}]}`
	rs := NewRunState("spec-prior", "proj-prior")
	rs.PriorReviewFindings = json.RawMessage(payload)

	if err := s.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	loaded, err := s.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}

	want := compactJSON(t, []byte(payload))
	got := compactJSON(t, loaded.PriorReviewFindings)
	if !bytes.Equal(want, got) {
		t.Fatalf("prior review findings mismatch: got %q want %q", got, want)
	}
}

func TestRunStore_SaveLoad_EmptyPriorReviewFindingsOmitted(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-empty", "proj-empty")

	if err := s.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(s.RunDir(rs.RunID), "run.json"))
	if err != nil {
		t.Fatalf("read run file: %v", err)
	}
	if strings.Contains(string(data), `"prior_review_findings"`) {
		t.Fatalf("run.json should omit prior_review_findings when empty, got %s", string(data))
	}

	loaded, err := s.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if loaded.PriorReviewFindings != nil {
		t.Fatalf("expected PriorReviewFindings to remain nil, got %q", loaded.PriorReviewFindings)
	}
}

func TestStore_RunStateThrashFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	rs := NewRunState("spec-thrash", "proj-thrash")
	fingerprint := "planner.go\x00buildFixPlanPrompt lacks X"
	rs.ReviewThrashCounts = map[string]int{fingerprint: 2}
	escalated := "review:spec_alignment:error:planner.go:buildFixPlanPrompt lacks X"
	rs.ReviewEscalatedFailures = []string{escalated}

	if err := s.Save(rs); err != nil {
		t.Fatalf("save run state: %v", err)
	}

	loaded, err := s.Get(rs.RunID)
	if err != nil {
		t.Fatalf("get run state: %v", err)
	}

	if got := loaded.ReviewThrashCounts[fingerprint]; got != 2 {
		t.Fatalf("expected thrash count=2, got %d", got)
	}
	if len(loaded.ReviewEscalatedFailures) != 1 || loaded.ReviewEscalatedFailures[0] != escalated {
		t.Fatalf("unexpected escalated failures: %v", loaded.ReviewEscalatedFailures)
	}
}

func TestStore_ResumeWithoutThrashFields(t *testing.T) {
	const tmpl = `{
		"run_id": "%s",
		"spec_id": "spec-resume",
		"project_id": "proj-resume",
		"status": "running",
		"cycle": 2,
		"started_at": "2026-03-24T00:00:00Z",
		"tasks": [],
		"accumulated_cost": 0,
		"final_validation_passed": false,
		"final_review_passed": false,
		"final_acceptance_passed": false,
		"contracts_written": false,
		"scenario_tests_written": false
	}`

	dir := t.TempDir()
	s := NewStore(dir)
	runID := "resume-run"
	runDir := s.RunDir(runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}

	payload := fmt.Sprintf(tmpl, runID)
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), []byte(payload), 0o644); err != nil {
		t.Fatalf("write legacy run: %v", err)
	}

	loaded, err := s.Get(runID)
	if err != nil {
		t.Fatalf("get legacy run: %v", err)
	}

	if loaded.ReviewThrashCounts == nil {
		t.Fatal("ReviewThrashCounts should be non-nil after resume")
	}
	if len(loaded.ReviewThrashCounts) != 0 {
		t.Fatalf("expected empty ReviewThrashCounts, got %v", loaded.ReviewThrashCounts)
	}
	if loaded.ReviewEscalatedFailures == nil {
		t.Fatal("ReviewEscalatedFailures should be non-nil after resume")
	}
	if len(loaded.ReviewEscalatedFailures) != 0 {
		t.Fatalf("expected empty ReviewEscalatedFailures, got %v", loaded.ReviewEscalatedFailures)
	}
}

func compactJSON(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, payload); err != nil {
		t.Fatalf("compact payload: %v", err)
	}
	return buf.Bytes()
}

func TestResetForNewCycle(t *testing.T) {
	rs := NewRunState("spec-1", "proj-1")
	rs.FinalValidationPassed = true
	rs.FinalReviewPassed = true
	rs.FinalAcceptancePassed = true
	rs.ReviewFindings = []string{"finding-1"}
	rs.AcceptanceResults = []string{"result-1"}
	rs.ScenarioTestsWritten = true
	rs.FailureHistory = map[string]int{"error1": 1, "error2": 2}

	ResetForNewCycle(rs)

	if rs.FinalValidationPassed {
		t.Fatal("FinalValidationPassed should be reset to false")
	}
	if rs.FinalReviewPassed {
		t.Fatal("FinalReviewPassed should be reset to false")
	}
	if rs.FinalAcceptancePassed {
		t.Fatal("FinalAcceptancePassed should be reset to false")
	}
	if len(rs.ReviewFindings) != 0 {
		t.Fatal("ReviewFindings should be reset to empty slice")
	}
	if len(rs.AcceptanceResults) != 0 {
		t.Fatal("AcceptanceResults should be reset to empty slice")
	}
	// ScenarioTestsWritten and FailureHistory should NOT be reset
	if !rs.ScenarioTestsWritten {
		t.Fatal("ScenarioTestsWritten should NOT be reset")
	}
	if len(rs.FailureHistory) != 2 {
		t.Fatal("FailureHistory should NOT be reset")
	}
	if rs.FailureHistory["error1"] != 1 || rs.FailureHistory["error2"] != 2 {
		t.Fatal("FailureHistory values should remain unchanged")
	}
}
