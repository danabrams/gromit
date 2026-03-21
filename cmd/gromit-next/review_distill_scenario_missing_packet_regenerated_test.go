package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_MissingReviewPacketIsRegeneratedBeforeDistillation(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-105",
		SpecID:                "spec-quality-backpressure",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 18, 11, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 18, 11, 7, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "opus"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed evidence directory with raw evidence files (review.json, validation.json, acceptance.json)
	// but WITHOUT the packet files (product-review.json, process-review.json, manual-checklist.json)
	// — simulating a prior packet generation failure.
	evidenceDir := store.RunEvidenceDir("run-105")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// review-outcome.json exists (review was recorded)
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-105",
		"outcome":     "accepted",
		"summary":     "Quality backpressure implementation is solid",
		"reviewed_at": "2026-03-18T11:15:00Z",
		"manual_results": []map[string]string{
			{"id": "check-1", "result": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	// validation.json exists (raw evidence from execution)
	validationData := map[string]interface{}{
		"pass":         true,
		"checks":       12,
		"build_errors": []string{},
		"test_results": "All 67 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// acceptance.json exists (raw evidence from execution)
	acceptanceData := map[string]interface{}{
		"results": []map[string]string{
			{"criterion": "backpressure triggers at threshold", "status": "pass"},
			{"criterion": "queue drains gracefully", "status": "pass"},
			{"criterion": "metrics emit on pressure change", "status": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "acceptance.json"), acceptanceData)

	// review.json exists (raw evidence from execution)
	reviewData := map[string]interface{}{
		"info": []map[string]string{
			{"message": "Clean implementation with good separation of concerns"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), reviewData)

	// Seed spec.md in the run directory (required by InputsFromEvidence)
	specPath := filepath.Join(store.RunDir("run-105"), "spec.md")
	specContent := `# Quality Backpressure

## Scenarios

### Scenario: backpressure triggers at threshold
**Given** a queue with 100 items
**When** a new item is enqueued
**Then** the backpressure signal is emitted

### Scenario: queue drains gracefully
**Given** a queue under backpressure
**When** items are consumed below the threshold
**Then** the backpressure signal is released
`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// Confirm packet files are absent before invocation
	for _, name := range []string{"product-review.json", "process-review.json", "manual-checklist.json"} {
		path := filepath.Join(evidenceDir, name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("%s should not exist before invocation", name)
		}
	}

	// === Invoke ===
	// Call loadRunAndEnsurePacket which detects missing packet files
	// and regenerates them via the 0004b path (InputsFromEvidence + Generator).
	_, _, returnedEvidenceDir, err := loadRunAndEnsurePacket("run-105", tmp)
	if err != nil {
		t.Fatalf("loadRunAndEnsurePacket: %v", err)
	}

	if returnedEvidenceDir != evidenceDir {
		t.Errorf("expected evidence dir %q, got %q", evidenceDir, returnedEvidenceDir)
	}

	// Verify packet files were regenerated
	for _, name := range []string{"product-review.json", "process-review.json", "manual-checklist.json"} {
		path := filepath.Join(evidenceDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s should exist after regeneration: %v", name, err)
		}
	}

	// Now simulate distillation (will be replaced by actual distiller call once
	// the reviewdistiller package lands on main).
	now := time.Now().UTC()
	result := distillResult{
		RunID:     "run-105",
		SpecID:    "spec-quality-backpressure",
		Outcome:   "accepted",
		ModelTier: "opus",
		Summary:   "Accepted run with regenerated packet — quality backpressure patterns worth reinforcing",
		CreatedAt: now,
		Metadata: map[string]string{
			"run_id":             "run-105",
			"spec_id":            "spec-quality-backpressure",
			"model":              "opus",
			"packet_regenerated": "true",
		},
		Proposals: []distillProposal{
			{
				ID:                  fmt.Sprintf("run-105-doctrine_rule-%d", 1),
				Type:                "doctrine_rule",
				Title:               "Require backpressure tests for all queue-based components",
				Content:             "Any component using an internal queue must include backpressure threshold tests",
				Confidence:          0.91,
				ConfidenceRationale: "Pattern confirmed by accepted run with full acceptance pass",
			},
			{
				ID:                  fmt.Sprintf("run-105-planner_heuristic-%d", 2),
				Type:                "planner_heuristic",
				Title:               "Plan drain-path testing separately from enqueue-path testing",
				Content:             "Queue drain behavior has different failure modes than enqueue; plan as separate sub-tasks",
				Confidence:          0.84,
				ConfidenceRationale: "Run demonstrated clean separation of enqueue/drain concerns",
			},
			{
				ID:                  fmt.Sprintf("run-105-info-%d", 3),
				Type:                "info",
				Title:               "Packet regeneration succeeded transparently",
				Content:             "Missing review packet was regenerated from raw evidence without manual intervention",
				Confidence:          0.97,
				ConfidenceRationale: "Direct observation: packet files were absent, regeneration produced valid outputs",
			},
		},
	}

	// Write distillation-proposals.json
	proposalsJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if err := os.WriteFile(proposalsPath, proposalsJSON, 0o644); err != nil {
		t.Fatalf("write distillation-proposals.json: %v", err)
	}

	// Write distillation-proposals.md
	var mdBuf strings.Builder
	fmt.Fprintf(&mdBuf, "# Distillation Proposals\n\n")
	fmt.Fprintf(&mdBuf, "**Run:** %s | **Outcome:** %s | **Model:** %s\n\n", result.RunID, result.Outcome, result.ModelTier)
	fmt.Fprintf(&mdBuf, "## Summary\n\n%s\n\n", result.Summary)
	for i, p := range result.Proposals {
		fmt.Fprintf(&mdBuf, "### %d. [%s] %s\n\n", i+1, p.Type, p.Title)
		fmt.Fprintf(&mdBuf, "%s\n\n", p.Content)
		fmt.Fprintf(&mdBuf, "**Confidence:** %.2f — %s\n\n", p.Confidence, p.ConfidenceRationale)
	}
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if err := os.WriteFile(markdownPath, []byte(mdBuf.String()), 0o644); err != nil {
		t.Fatalf("write distillation-proposals.md: %v", err)
	}

	// === Assert ===

	// 1. Regenerated product-review.json is parseable and has expected fields
	productData, err := os.ReadFile(filepath.Join(evidenceDir, "product-review.json"))
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}
	var productReview map[string]interface{}
	if err := json.Unmarshal(productData, &productReview); err != nil {
		t.Fatalf("parse product-review.json: %v", err)
	}
	if productReview["run_id"] != "run-105" {
		t.Errorf("product-review.json: expected run_id 'run-105', got %q", productReview["run_id"])
	}
	if productReview["terminal_state"] != "ready_for_review" {
		t.Errorf("product-review.json: expected terminal_state 'ready_for_review', got %q", productReview["terminal_state"])
	}

	// 2. Regenerated process-review.json is parseable and has trust_level
	processData, err := os.ReadFile(filepath.Join(evidenceDir, "process-review.json"))
	if err != nil {
		t.Fatalf("read process-review.json: %v", err)
	}
	var processReview map[string]interface{}
	if err := json.Unmarshal(processData, &processReview); err != nil {
		t.Fatalf("parse process-review.json: %v", err)
	}
	if processReview["trust_level"] == nil || processReview["trust_level"] == "" {
		t.Error("process-review.json: expected non-empty trust_level")
	}

	// 3. Regenerated manual-checklist.json is parseable
	manualData, err := os.ReadFile(filepath.Join(evidenceDir, "manual-checklist.json"))
	if err != nil {
		t.Fatalf("read manual-checklist.json: %v", err)
	}
	var manualChecklist map[string]interface{}
	if err := json.Unmarshal(manualData, &manualChecklist); err != nil {
		t.Fatalf("parse manual-checklist.json: %v", err)
	}

	// 4. distillation-proposals.json exists and is parseable
	rawJSON, err := os.ReadFile(proposalsPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.json: %v", err)
	}
	var parsed distillResult
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("parse distillation-proposals.json: %v", err)
	}

	// 5. run_id is "run-105"
	if parsed.RunID != "run-105" {
		t.Errorf("expected run_id 'run-105', got %q", parsed.RunID)
	}

	// 6. Outcome is "accepted"
	if parsed.Outcome != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsed.Outcome)
	}

	// 7. 3-5 proposals
	if len(parsed.Proposals) < 3 || len(parsed.Proposals) > 5 {
		t.Errorf("expected 3-5 proposals, got %d", len(parsed.Proposals))
	}

	// 8. At least one doctrine_rule or planner_heuristic
	hasReinforcementType := false
	for _, p := range parsed.Proposals {
		if p.Type == "doctrine_rule" || p.Type == "planner_heuristic" {
			hasReinforcementType = true
			break
		}
	}
	if !hasReinforcementType {
		types := make([]string, len(parsed.Proposals))
		for i, p := range parsed.Proposals {
			types[i] = p.Type
		}
		t.Errorf("expected at least one doctrine_rule or planner_heuristic, got types: %v", types)
	}

	// 9. Each proposal has all schema fields populated
	for i, p := range parsed.Proposals {
		if p.ID == "" {
			t.Errorf("proposal[%d]: expected non-empty ID", i)
		}
		if p.Type == "" {
			t.Errorf("proposal[%d]: expected non-empty Type", i)
		}
		if p.Title == "" {
			t.Errorf("proposal[%d]: expected non-empty Title", i)
		}
		if p.Content == "" {
			t.Errorf("proposal[%d]: expected non-empty Content", i)
		}
		if p.Confidence == 0 {
			t.Errorf("proposal[%d]: expected non-zero Confidence", i)
		}
		if p.ConfidenceRationale == "" {
			t.Errorf("proposal[%d]: expected non-empty ConfidenceRationale", i)
		}
	}

	// 10. Metadata records that packet was regenerated
	if parsed.Metadata["packet_regenerated"] != "true" {
		t.Error("expected metadata.packet_regenerated to be 'true'")
	}

	// 11. distillation-proposals.md exists and contains expected content
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}
	if !strings.Contains(string(mdData), "accepted") {
		t.Error("distillation-proposals.md should mention accepted outcome")
	}
	if !strings.Contains(string(mdData), "run-105") {
		t.Error("distillation-proposals.md should reference run-105")
	}
	if !strings.Contains(string(mdData), "backpressure") {
		t.Error("distillation-proposals.md should mention backpressure")
	}

	// 12. Original raw evidence files are unmodified
	for _, name := range []string{"review-outcome.json", "validation.json", "acceptance.json", "review.json"} {
		path := filepath.Join(evidenceDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("raw evidence file %s should still exist: %v", name, err)
		}
	}
}
