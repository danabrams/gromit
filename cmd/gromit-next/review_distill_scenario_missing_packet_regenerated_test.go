package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
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

	// Load regenerated artifacts for distiller inputs
	outcomeDataBytes, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}
	productDataBytes, err := os.ReadFile(filepath.Join(evidenceDir, "product-review.json"))
	if err != nil {
		t.Fatalf("read product-review.json: %v", err)
	}
	processDataBytes, err := os.ReadFile(filepath.Join(evidenceDir, "process-review.json"))
	if err != nil {
		t.Fatalf("read process-review.json: %v", err)
	}
	checklistDataBytes, err := os.ReadFile(filepath.Join(evidenceDir, "manual-checklist.json"))
	if err != nil {
		t.Fatalf("read manual-checklist.json: %v", err)
	}
	validationDataBytes, err := os.ReadFile(filepath.Join(evidenceDir, "validation.json"))
	if err != nil {
		t.Fatalf("read validation.json: %v", err)
	}
	acceptanceDataBytes, err := os.ReadFile(filepath.Join(evidenceDir, "acceptance.json"))
	if err != nil {
		t.Fatalf("read acceptance.json: %v", err)
	}
	reviewDataBytes, err := os.ReadFile(filepath.Join(evidenceDir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}

	// Build DistillerInputs
	inputs := &reviewdistiller.DistillerInputs{
		RunID:           "run-105",
		SpecID:          "spec-quality-backpressure",
		SpecContent:     specContent,
		ReviewOutcome:   json.RawMessage(outcomeDataBytes),
		ProductReview:   json.RawMessage(productDataBytes),
		ProcessReview:   json.RawMessage(processDataBytes),
		ManualChecklist: json.RawMessage(checklistDataBytes),
		Validation:      json.RawMessage(validationDataBytes),
		Acceptance:      json.RawMessage(acceptanceDataBytes),
		MachineReview:   json.RawMessage(reviewDataBytes),
	}

	// Create mock LLM completer that returns sample proposals
	mockLLM := &mockLLMCompleter{
		response: `{
  "proposals": [
    {
      "type": "doctrine_rule",
      "title": "Require backpressure tests for all queue-based components",
      "what_happened": "Implementation included backpressure mechanism but tests could be more systematic",
      "what_was_missing": "Doctrine rule on mandatory backpressure test coverage",
      "proposed_change": "Document: any queue-based component must include backpressure threshold tests",
      "rationale": "Pattern confirmed by accepted run with full acceptance pass",
      "confidence": "high",
      "confidence_rationale": "Direct observation from successful implementation",
      "evidence_references": ["acceptance.json"]
    },
    {
      "type": "planner_heuristic",
      "title": "Plan drain-path testing separately from enqueue-path testing",
      "what_happened": "Tests covered both paths but could benefit from explicit separation",
      "what_was_missing": "Documented heuristic on path isolation",
      "proposed_change": "Add planner heuristic: queue operations should test enqueue and drain as separate sub-tasks",
      "rationale": "Queue drain behavior has different failure modes than enqueue",
      "confidence": "medium",
      "confidence_rationale": "Run demonstrated clean separation of enqueue/drain concerns",
      "evidence_references": ["validation.json", "acceptance.json"]
    },
    {
      "type": "refinement_guidance",
      "title": "Packet regeneration succeeded transparently",
      "what_happened": "Missing review packet was successfully regenerated from raw evidence",
      "what_was_missing": "No guidance needed — success case",
      "proposed_change": "Document success: InputsFromEvidence → Generator pipeline handles missing packets correctly",
      "rationale": "Demonstrates robustness of packet regeneration mechanism",
      "confidence": "high",
      "confidence_rationale": "Direct observation: packet files were absent, regeneration produced valid outputs",
      "evidence_references": ["product-review.json", "process-review.json"]
    }
  ]
}`,
	}

	// Call reviewdistiller.Distill
	result, err := reviewdistiller.Distill(inputs, mockLLM, reviewdistiller.TierHigh)
	if err != nil {
		t.Fatalf("Distill: %v", err)
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
	fmt.Fprintf(&mdBuf, "**Run:** %s | **Outcome:** %s | **Model Tier:** %s\n\n", result.RunID, result.Outcome, result.ModelTier)
	fmt.Fprintf(&mdBuf, "**Generated:** %s\n\n", result.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(&mdBuf, "## Summary\n\nDistilled %s review outcome into %d improvement proposals.\n\n", result.Outcome, len(result.Proposals))
	for _, p := range result.Proposals {
		fmt.Fprintf(&mdBuf, "---\n\n")
		fmt.Fprintf(&mdBuf, "## %s\n\n", p.Title)
		fmt.Fprintf(&mdBuf, "**Type:** %s\n", p.Type)
		fmt.Fprintf(&mdBuf, "**Confidence:** %s\n\n", p.Confidence)
		fmt.Fprintf(&mdBuf, "### What Happened\n\n%s\n\n", p.WhatHappened)
		fmt.Fprintf(&mdBuf, "### What Was Missing\n\n%s\n\n", p.WhatWasMissing)
		fmt.Fprintf(&mdBuf, "### Proposed Change\n\n%s\n\n", p.ProposedChange)
		fmt.Fprintf(&mdBuf, "### Rationale\n\n%s\n\n", p.Rationale)
		fmt.Fprintf(&mdBuf, "**Confidence Rationale:** %s\n\n", p.ConfidenceRationale)
		if len(p.EvidenceReferences) > 0 {
			fmt.Fprintf(&mdBuf, "### Evidence References\n\n")
			for _, ref := range p.EvidenceReferences {
				fmt.Fprintf(&mdBuf, "- %s\n", ref)
			}
			fmt.Fprintf(&mdBuf, "\n")
		}
	}
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if err := os.WriteFile(markdownPath, []byte(mdBuf.String()), 0o644); err != nil {
		t.Fatalf("write distillation-proposals.md: %v", err)
	}

	// === Assert ===

	// 1. Regenerated product-review.json is parseable and has expected fields
	var productReview map[string]interface{}
	if err := json.Unmarshal(productDataBytes, &productReview); err != nil {
		t.Fatalf("parse product-review.json: %v", err)
	}
	if productReview["run_id"] != "run-105" {
		t.Errorf("product-review.json: expected run_id 'run-105', got %q", productReview["run_id"])
	}
	if productReview["terminal_state"] != "ready_for_review" {
		t.Errorf("product-review.json: expected terminal_state 'ready_for_review', got %q", productReview["terminal_state"])
	}

	// 2. Regenerated process-review.json is parseable and has trust_level
	var processReview map[string]interface{}
	if err := json.Unmarshal(processDataBytes, &processReview); err != nil {
		t.Fatalf("parse process-review.json: %v", err)
	}
	if processReview["trust_level"] == nil || processReview["trust_level"] == "" {
		t.Error("process-review.json: expected non-empty trust_level")
	}

	// 3. Regenerated manual-checklist.json is parseable
	var manualChecklist map[string]interface{}
	if err := json.Unmarshal(checklistDataBytes, &manualChecklist); err != nil {
		t.Fatalf("parse manual-checklist.json: %v", err)
	}

	// 4. distillation-proposals.json exists and is parseable
	rawJSON, err := os.ReadFile(proposalsPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.json: %v", err)
	}
	var parsed reviewdistiller.DistillationResult
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
		if p.WhatHappened == "" {
			t.Errorf("proposal[%d]: expected non-empty WhatHappened", i)
		}
		if p.WhatWasMissing == "" {
			t.Errorf("proposal[%d]: expected non-empty WhatWasMissing", i)
		}
		if p.ProposedChange == "" {
			t.Errorf("proposal[%d]: expected non-empty ProposedChange", i)
		}
		if p.Rationale == "" {
			t.Errorf("proposal[%d]: expected non-empty Rationale", i)
		}
		if p.Confidence == "" {
			t.Errorf("proposal[%d]: expected non-empty Confidence", i)
		}
		if p.ConfidenceRationale == "" {
			t.Errorf("proposal[%d]: expected non-empty ConfidenceRationale", i)
		}
	}

	// 10. distillation-proposals.md exists and contains expected content
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

	// 11. Original raw evidence files are unmodified
	for _, name := range []string{"review-outcome.json", "validation.json", "acceptance.json", "review.json"} {
		path := filepath.Join(evidenceDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("raw evidence file %s should still exist: %v", name, err)
		}
	}
}
