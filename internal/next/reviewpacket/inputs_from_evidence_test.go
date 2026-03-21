package reviewpacket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestInputsFromEvidence_SuccessfulReconstruction(t *testing.T) {
	// Scenario: InputsFromEvidence reconstructs Inputs from complete evidence directory
	// Given: a directory with validation.json, acceptance.json, review.json, and a spec file with valid content
	// When: InputsFromEvidence is called
	// Then: it returns an Inputs struct with all fields populated correctly

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	specContent := "# Test Spec\n\nThis is a test specification."
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write validation.json in the real validator.FinalResult format:
	// "pass" (not "passed"), results nested under always_run/project_checks.
	validationData := testValidationJSON(true, 8, 4)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	// Write acceptance.json in results-array format
	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
			{"status": "pass"},
			{"status": "pass"},
			{"status": "pass"},
			{"status": "pass"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Write review.json
	reviewData := map[string]interface{}{
		"info": []interface{}{
			map[string]interface{}{"message": "clean code"},
		},
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create RunState with test data
	run := &runstore.RunState{
		RunID:          "test-run-001",
		SpecID:         "spec-0001",
		Status:         "ready_for_review",
		ReviewFindings: []string{"info"},
		Cycle:          2,
		TotalReplans:   2,
		FailureHistory: map[string]int{},
		TaskLineage:    map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	// Verify Inputs were reconstructed correctly
	if inputs.RunID != "test-run-001" {
		t.Errorf("RunID = %q, want %q", inputs.RunID, "test-run-001")
	}
	if inputs.SpecTitle != "spec-0001" {
		t.Errorf("SpecTitle = %q, want %q", inputs.SpecTitle, "spec-0001")
	}
	if inputs.SpecContent != specContent {
		t.Errorf("SpecContent = %q, want %q", inputs.SpecContent, specContent)
	}
	if inputs.TerminalState != "ready_for_review" {
		t.Errorf("TerminalState = %q, want %q", inputs.TerminalState, "ready_for_review")
	}

	// Verify ValidationResult
	if !inputs.ValidationResult.Passed {
		t.Errorf("ValidationResult.Passed = false, want true")
	}
	if inputs.ValidationResult.Checks != 12 {
		t.Errorf("ValidationResult.Checks = %d, want 12", inputs.ValidationResult.Checks)
	}

	// Verify AcceptanceResult
	if inputs.AcceptanceResult.Passed != 5 {
		t.Errorf("AcceptanceResult.Passed = %d, want 5", inputs.AcceptanceResult.Passed)
	}
	if inputs.AcceptanceResult.Failed != 0 {
		t.Errorf("AcceptanceResult.Failed = %d, want 0", inputs.AcceptanceResult.Failed)
	}

	// Verify ReviewFindings extracted correctly
	if _, ok := inputs.ReviewFindings["info"]; !ok {
		t.Errorf("ReviewFindings missing 'info' category")
	}
	if len(inputs.ReviewFindings["info"]) != 1 {
		t.Errorf("ReviewFindings['info'] length = %d, want 1", len(inputs.ReviewFindings["info"]))
	}

	// Verify DegradedFlags (no diff_unavailable in review.json, so empty)
	if len(inputs.DegradedFlags) != 0 {
		t.Errorf("DegradedFlags length = %d, want 0", len(inputs.DegradedFlags))
	}

	// Verify RepairCycles
	if inputs.RepairCycles != 2 {
		t.Errorf("RepairCycles = %d, want 2", inputs.RepairCycles)
	}
}

func TestInputsFromEvidence_MissingValidationJson(t *testing.T) {
	// Scenario: validation.json is missing from evidence directory
	// Given: an evidence directory without validation.json
	// When: InputsFromEvidence is called
	// Then: it returns an error indicating validation.json is missing

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write acceptance.json in results-array format
	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Write review.json
	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{RunID: "test-run", Status: "ready_for_review"}

	_, err := InputsFromEvidence(tempDir, specPath, run)
	if err == nil {
		t.Error("InputsFromEvidence() error = nil, want error for missing validation.json")
	}
	if err != nil && !strings.Contains(err.Error(), "validation.json") {
		t.Errorf("error message does not mention validation.json: %v", err)
	}
}

func TestInputsFromEvidence_MissingAcceptanceJson(t *testing.T) {
	// Scenario: acceptance.json is missing from evidence directory
	// Given: an evidence directory without acceptance.json
	// When: InputsFromEvidence is called
	// Then: it returns an error indicating acceptance.json is missing

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write validation.json
	validationData := testValidationJSON(true, 3, 2)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	// Write review.json
	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{RunID: "test-run", Status: "ready_for_review"}

	_, err := InputsFromEvidence(tempDir, specPath, run)
	if err == nil {
		t.Error("InputsFromEvidence() error = nil, want error for missing acceptance.json")
	}
	if err != nil && !strings.Contains(err.Error(), "acceptance.json") {
		t.Errorf("error message does not mention acceptance.json: %v", err)
	}
}

func TestInputsFromEvidence_MissingSpecFile(t *testing.T) {
	// Scenario: spec file doesn't exist
	// Given: a spec file path that doesn't exist
	// When: InputsFromEvidence is called
	// Then: it returns an error indicating the spec file is missing

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "nonexistent.md")

	// Write evidence files
	validationData := testValidationJSON(true, 3, 2)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{"results": []map[string]interface{}{}}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{RunID: "test-run", Status: "ready_for_review"}

	_, err := InputsFromEvidence(tempDir, specPath, run)
	if err == nil {
		t.Error("InputsFromEvidence() error = nil, want error for missing spec file")
	}
	if err != nil && !strings.Contains(err.Error(), "read spec") {
		t.Errorf("error message does not mention spec: %v", err)
	}
}

func TestInputsFromEvidence_MissingReviewJson(t *testing.T) {
	// Scenario: review.json is missing from evidence directory
	// Given: an evidence directory without review.json
	// When: InputsFromEvidence is called
	// Then: it returns an error indicating review.json is missing

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write validation.json in real format
	validationData := testValidationJSON(true, 3, 2)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	// Write acceptance.json in results-array format
	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	run := &runstore.RunState{RunID: "test-run", Status: "ready_for_review"}

	_, err := InputsFromEvidence(tempDir, specPath, run)
	if err == nil {
		t.Error("InputsFromEvidence() error = nil, want error for missing review.json")
	}
	if err != nil && !strings.Contains(err.Error(), "review.json") {
		t.Errorf("error message does not mention review.json: %v", err)
	}
}

func TestInputsFromEvidence_InvalidValidationJson(t *testing.T) {
	// Scenario: validation.json contains invalid JSON
	// Given: validation.json with malformed JSON
	// When: InputsFromEvidence is called
	// Then: it returns an error indicating JSON unmarshal failure

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write invalid validation.json
	if err := os.WriteFile(filepath.Join(tempDir, "validation.json"), []byte("{invalid json}"), 0o644); err != nil {
		t.Fatalf("write invalid validation.json: %v", err)
	}

	// Write acceptance.json in results-array format
	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Write review.json
	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{RunID: "test-run", Status: "ready_for_review"}

	_, err := InputsFromEvidence(tempDir, specPath, run)
	if err == nil {
		t.Error("InputsFromEvidence() error = nil, want error for invalid validation.json")
	}
	if err != nil && !strings.Contains(err.Error(), "unmarshal validation.json") {
		t.Errorf("error message does not mention unmarshal validation.json: %v", err)
	}
}

func TestInputsFromEvidence_InvalidAcceptanceJson(t *testing.T) {
	// Scenario: acceptance.json contains invalid JSON
	// Given: acceptance.json with malformed JSON
	// When: InputsFromEvidence is called
	// Then: it returns an error indicating JSON unmarshal failure

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write validation.json
	validationData := testValidationJSON(true, 3, 2)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	// Write invalid acceptance.json
	if err := os.WriteFile(filepath.Join(tempDir, "acceptance.json"), []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("write invalid acceptance.json: %v", err)
	}

	// Write review.json
	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{RunID: "test-run", Status: "ready_for_review"}

	_, err := InputsFromEvidence(tempDir, specPath, run)
	if err == nil {
		t.Error("InputsFromEvidence() error = nil, want error for invalid acceptance.json")
	}
	if err != nil && !strings.Contains(err.Error(), "unmarshal acceptance.json") {
		t.Errorf("error message does not mention unmarshal acceptance.json: %v", err)
	}
}

func TestInputsFromEvidence_InvalidReviewJson(t *testing.T) {
	// Scenario: review.json contains invalid JSON
	// Given: review.json with malformed JSON
	// When: InputsFromEvidence is called
	// Then: it returns an error indicating JSON unmarshal failure

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write validation.json
	validationData := testValidationJSON(true, 3, 2)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	// Write acceptance.json in results-array format
	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Write invalid review.json
	if err := os.WriteFile(filepath.Join(tempDir, "review.json"), []byte("[unclosed array"), 0o644); err != nil {
		t.Fatalf("write invalid review.json: %v", err)
	}

	run := &runstore.RunState{RunID: "test-run", Status: "ready_for_review"}

	_, err := InputsFromEvidence(tempDir, specPath, run)
	if err == nil {
		t.Error("InputsFromEvidence() error = nil, want error for invalid review.json")
	}
	if err != nil && !strings.Contains(err.Error(), "unmarshal review.json") {
		t.Errorf("error message does not mention unmarshal review.json: %v", err)
	}
}

func TestInputsFromEvidence_ExtractRunMetadata(t *testing.T) {
	// Scenario: Run metadata (RunID, TerminalState, DegradedFlags, RepairCycles) are correctly extracted
	// Given: a RunState with specific metadata and an evidence directory
	// When: InputsFromEvidence is called
	// Then: the returned Inputs contain the correct metadata values

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write evidence files
	validationData := testValidationJSON(true, 5, 3)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
			{"status": "pass"},
			{"status": "pass"},
			{"status": "fail"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	reviewData := map[string]interface{}{
		"diff_unavailable": true,
		"info":             []interface{}{},
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create RunState with specific values
	run := &runstore.RunState{
		RunID:          "specific-run-id-123",
		SpecID:         "spec-0042",
		Status:         "blocked",
		Cycle:          5,
		TotalReplans:   4,
		FailureHistory: map[string]int{},
		TaskLineage:    map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	// Verify RunID
	if inputs.RunID != "specific-run-id-123" {
		t.Errorf("RunID = %q, want %q", inputs.RunID, "specific-run-id-123")
	}

	// Verify TerminalState (from Status)
	if inputs.TerminalState != "blocked" {
		t.Errorf("TerminalState = %q, want %q", inputs.TerminalState, "blocked")
	}

	// Verify DegradedFlags derived from review.json diff_unavailable
	if len(inputs.DegradedFlags) != 1 {
		t.Errorf("len(DegradedFlags) = %d, want 1", len(inputs.DegradedFlags))
	}
	if !contains(inputs.DegradedFlags, "diff_unavailable") {
		t.Errorf("DegradedFlags missing 'diff_unavailable': %v", inputs.DegradedFlags)
	}

	// Verify RepairCycles (uses Cycle, not TotalReplans)
	if inputs.RepairCycles != 5 {
		t.Errorf("RepairCycles = %d, want 5", inputs.RepairCycles)
	}
}

func TestInputsFromEvidence_RepeatedFailureDetectionViaTaskLineageOnly(t *testing.T) {
	// Scenario: RepeatedFailure is only detected via TaskLineage (not FailureHistory)
	// Given: a RunState with FailureHistory containing repeated failures but empty TaskLineage
	// When: InputsFromEvidence is called
	// Then: the returned Inputs has RepeatedFailure = false (FailureHistory is ignored)

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write evidence files
	validationData := testValidationJSON(false, 2, 1)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "fail"},
			{"status": "fail"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create RunState with repeated failures in FailureHistory but empty TaskLineage
	run := &runstore.RunState{
		RunID:  "repeated-run",
		Status: "blocked",
		FailureHistory: map[string]int{
			"type_error":   2,
			"syntax_error": 1,
		},
		TaskLineage: map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	// FailureHistory is no longer checked; only TaskLineage matters
	if inputs.RepeatedFailure {
		t.Errorf("RepeatedFailure = true, want false when only FailureHistory has count > 1 (TaskLineage is empty)")
	}
}

func TestInputsFromEvidence_RepeatedFailureDetectionViaTaskLineage(t *testing.T) {
	// Scenario: RepeatedFailure is set when TaskLineage contains entries with ConsecutiveFails > 1
	// Given: a RunState with TaskLineage containing consecutive failures
	// When: InputsFromEvidence is called
	// Then: the returned Inputs has RepeatedFailure = true

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write evidence files
	validationData := testValidationJSON(false, 1, 1)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "fail"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create RunState with consecutive failures in taskLineage
	run := &runstore.RunState{
		RunID:          "tasklineage-run",
		Status:         "blocked",
		FailureHistory: map[string]int{},
		TaskLineage: map[string]runstore.TaskLineageEntry{
			"task-1": {
				ConsecutiveFails: 3,
				ChainIDs:         []string{"id1", "id2", "id3"},
			},
		},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	if !inputs.RepeatedFailure {
		t.Errorf("RepeatedFailure = false, want true when TaskLineage has ConsecutiveFails > 1")
	}
}

func TestInputsFromEvidence_NoRepeatedFailure(t *testing.T) {
	// Scenario: RepeatedFailure is false when no history or lineage indicates repeated failures
	// Given: a RunState with empty FailureHistory and no consecutive fails in TaskLineage
	// When: InputsFromEvidence is called
	// Then: the returned Inputs has RepeatedFailure = false

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write evidence files
	validationData := testValidationJSON(true, 3, 2)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
			{"status": "pass"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create RunState without repeated failures
	run := &runstore.RunState{
		RunID:  "clean-run",
		Status: "ready_for_review",
		FailureHistory: map[string]int{
			"error_1": 1, // Only 1 occurrence
		},
		TaskLineage: map[string]runstore.TaskLineageEntry{
			"task-1": {
				ConsecutiveFails: 1, // Only 1 fail
				ChainIDs:         []string{"id1"},
			},
		},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	if inputs.RepeatedFailure {
		t.Errorf("RepeatedFailure = true, want false when no repeated failures detected")
	}
}

func TestInputsFromEvidence_EmptyDegradedFlags(t *testing.T) {
	// Scenario: DegradedFlags is empty array when review.json has no diff_unavailable field
	// Given: a review.json without diff_unavailable
	// When: InputsFromEvidence is called
	// Then: the returned Inputs has DegradedFlags as empty slice, not nil

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write evidence files
	validationData := testValidationJSON(true, 1, 0)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	reviewData := map[string]interface{}{"info": []interface{}{}}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create RunState with nil ReviewFindings
	run := &runstore.RunState{
		RunID:          "no-findings",
		Status:         "ready_for_review",
		ReviewFindings: nil,
		FailureHistory: map[string]int{},
		TaskLineage:    map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	if inputs.DegradedFlags == nil {
		t.Errorf("DegradedFlags is nil, want empty slice")
	}
	if len(inputs.DegradedFlags) != 0 {
		t.Errorf("len(DegradedFlags) = %d, want 0", len(inputs.DegradedFlags))
	}
}

func TestInputsFromEvidence_MultipleReviewFindingCategories(t *testing.T) {
	// Scenario: ReviewFindings with multiple categories are all extracted
	// Given: review.json with multiple finding categories (info, warning, blocking)
	// When: InputsFromEvidence is called
	// Then: all categories are present in ReviewFindings map

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write evidence files
	validationData := testValidationJSON(false, 3, 2)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
			{"status": "fail"},
			{"status": "fail"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Write review.json with multiple categories
	reviewData := map[string]interface{}{
		"diff_unavailable": false,
		"info": []interface{}{
			map[string]interface{}{"message": "code style suggestion"},
		},
		"warning": []interface{}{
			map[string]interface{}{"message": "potential issue"},
			map[string]interface{}{"message": "performance concern"},
		},
		"blocking": []interface{}{
			map[string]interface{}{"message": "build failed"},
		},
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{
		RunID:          "multi-findings",
		Status:         "blocked",
		FailureHistory: map[string]int{},
		TaskLineage:    map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	// Verify all categories are present
	categories := []string{"info", "warning", "blocking"}
	for _, cat := range categories {
		if _, ok := inputs.ReviewFindings[cat]; !ok {
			t.Errorf("ReviewFindings missing category %q", cat)
		}
	}

	// Verify counts
	if len(inputs.ReviewFindings["info"]) != 1 {
		t.Errorf("ReviewFindings['info'] length = %d, want 1", len(inputs.ReviewFindings["info"]))
	}
	if len(inputs.ReviewFindings["warning"]) != 2 {
		t.Errorf("ReviewFindings['warning'] length = %d, want 2", len(inputs.ReviewFindings["warning"]))
	}
	if len(inputs.ReviewFindings["blocking"]) != 1 {
		t.Errorf("ReviewFindings['blocking'] length = %d, want 1", len(inputs.ReviewFindings["blocking"]))
	}

	// Verify diff_unavailable is not included
	if _, ok := inputs.ReviewFindings["diff_unavailable"]; ok {
		t.Errorf("ReviewFindings should not contain diff_unavailable")
	}
}

func TestInputsFromEvidence_EmptyReviewFindings(t *testing.T) {
	// Scenario: ReviewFindings defaults to info category when review.json has no findings
	// Given: review.json with only diff_unavailable field (no actual findings)
	// When: InputsFromEvidence is called
	// Then: ReviewFindings contains default 'info' category with empty array

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write evidence files
	validationData := testValidationJSON(true, 2, 1)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Write review.json with only diff_unavailable (no findings)
	reviewData := map[string]interface{}{
		"diff_unavailable": true,
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{
		RunID:          "empty-findings",
		Status:         "ready_for_review",
		FailureHistory: map[string]int{},
		TaskLineage:    map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	// Verify default 'info' category exists
	if _, ok := inputs.ReviewFindings["info"]; !ok {
		t.Errorf("ReviewFindings missing default 'info' category")
	}
}

func TestInputsFromEvidence_RoundTripAcceptanceFormat(t *testing.T) {
	// Scenario: acceptance.json written by finalize stage format (results-array)
	// can be read back by evidence_loader with correct counts
	// Given: acceptance.json with results array containing pass/fail/unclear statuses
	// When: InputsFromEvidence is called
	// Then: AcceptanceResult contains correct counts parsed from results array

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	// Write spec file
	if err := os.WriteFile(specPath, []byte("# Round-trip Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Write validation.json
	validationData := testValidationJSON(true, 6, 4)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	// Write acceptance.json in the exact format that finalize.go produces
	// (results array with status field for each acceptance item)
	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
			{"status": "pass"},
			{"status": "pass"},
			{"status": "fail"},
			{"status": "unclear"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	// Write review.json
	reviewData := map[string]interface{}{
		"info": []interface{}{
			map[string]interface{}{"message": "test finding"},
		},
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{
		RunID:          "round-trip-test",
		Status:         "ready_for_review",
		FailureHistory: map[string]int{},
		TaskLineage:    map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	// Verify that counts match the results array
	if inputs.AcceptanceResult.Passed != 3 {
		t.Errorf("AcceptanceResult.Passed = %d, want 3", inputs.AcceptanceResult.Passed)
	}
	if inputs.AcceptanceResult.Failed != 1 {
		t.Errorf("AcceptanceResult.Failed = %d, want 1", inputs.AcceptanceResult.Failed)
	}
	if inputs.AcceptanceResult.Unclear != 1 {
		t.Errorf("AcceptanceResult.Unclear = %d, want 1", inputs.AcceptanceResult.Unclear)
	}
}

func TestInputsFromEvidence_DegradedFlagsFromDiffUnavailableTrue(t *testing.T) {
	// Scenario: DegradedFlags contains "diff_unavailable" when review.json has diff_unavailable: true
	// Given: review.json with diff_unavailable set to true
	// When: InputsFromEvidence is called
	// Then: DegradedFlags contains exactly ["diff_unavailable"]

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	validationData := testValidationJSON(true, 2, 1)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	reviewData := map[string]interface{}{
		"diff_unavailable": true,
		"info":             []interface{}{},
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{
		RunID:          "degraded-true",
		Status:         "ready_for_review",
		FailureHistory: map[string]int{},
		TaskLineage:    map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	if len(inputs.DegradedFlags) != 1 {
		t.Fatalf("len(DegradedFlags) = %d, want 1", len(inputs.DegradedFlags))
	}
	if inputs.DegradedFlags[0] != "diff_unavailable" {
		t.Errorf("DegradedFlags[0] = %q, want %q", inputs.DegradedFlags[0], "diff_unavailable")
	}
}

func TestInputsFromEvidence_DegradedFlagsFromDiffUnavailableFalse(t *testing.T) {
	// Scenario: DegradedFlags is empty when review.json has diff_unavailable: false
	// Given: review.json with diff_unavailable set to false
	// When: InputsFromEvidence is called
	// Then: DegradedFlags is empty

	tempDir := t.TempDir()
	specPath := filepath.Join(tempDir, "spec.md")

	if err := os.WriteFile(specPath, []byte("# Spec"), 0o644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	validationData := testValidationJSON(true, 2, 1)
	if err := writeJSON(tempDir, "validation.json", validationData); err != nil {
		t.Fatalf("write validation.json: %v", err)
	}

	acceptanceData := map[string]interface{}{
		"results": []map[string]interface{}{
			{"status": "pass"},
		},
	}
	if err := writeJSON(tempDir, "acceptance.json", acceptanceData); err != nil {
		t.Fatalf("write acceptance.json: %v", err)
	}

	reviewData := map[string]interface{}{
		"diff_unavailable": false,
		"info":             []interface{}{},
	}
	if err := writeJSON(tempDir, "review.json", reviewData); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	run := &runstore.RunState{
		RunID:          "degraded-false",
		Status:         "ready_for_review",
		FailureHistory: map[string]int{},
		TaskLineage:    map[string]runstore.TaskLineageEntry{},
	}

	inputs, err := InputsFromEvidence(tempDir, specPath, run)
	if err != nil {
		t.Fatalf("InputsFromEvidence() error = %v, want nil", err)
	}

	if len(inputs.DegradedFlags) != 0 {
		t.Errorf("len(DegradedFlags) = %d, want 0 when diff_unavailable is false", len(inputs.DegradedFlags))
	}
}

// testValidationJSON builds a validation.json fixture in the real validator.FinalResult format.
// The real format uses "pass" (not "passed") and nests results under always_run/project_checks.
func testValidationJSON(pass bool, alwaysRunCount, projectChecksCount int) map[string]interface{} {
	alwaysRunResults := make([]map[string]interface{}, alwaysRunCount)
	for i := range alwaysRunResults {
		alwaysRunResults[i] = map[string]interface{}{"pass": pass}
	}
	projectResults := make([]map[string]interface{}, projectChecksCount)
	for i := range projectResults {
		projectResults[i] = map[string]interface{}{"pass": pass}
	}
	return map[string]interface{}{
		"pass":           pass,
		"always_run":     map[string]interface{}{"results": alwaysRunResults},
		"project_checks": map[string]interface{}{"results": projectResults},
	}
}

// Helper function to check if a string contains a substring
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

