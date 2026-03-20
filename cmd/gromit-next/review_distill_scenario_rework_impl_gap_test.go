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

// reworkProposal extends the proposal schema with evidence_references.
type reworkProposal struct {
	ID                  string   `json:"id"`
	Type                string   `json:"type"`
	Title               string   `json:"title"`
	Content             string   `json:"content,omitempty"`
	Confidence          float64  `json:"confidence,omitempty"`
	ConfidenceRationale string   `json:"confidence_rationale,omitempty"`
	EvidenceReferences  []string `json:"evidence_references,omitempty"`
}

// reworkDistillResult represents the distillation output for rework scenario assertions.
type reworkDistillResult struct {
	RunID     string            `json:"run_id"`
	SpecID    string            `json:"spec_id"`
	Outcome   string            `json:"outcome"`
	ModelTier string            `json:"model_tier"`
	Summary   string            `json:"summary"`
	Proposals []reworkProposal  `json:"proposals"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func TestScenario_ReworkImplementationGapProducesGuardrailProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-102",
		SpecID:                "spec-keyboard-nav",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 16, 14, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 16, 14, 10, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: false,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "sonnet"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed evidence files
	evidenceDir := store.RunEvidenceDir("run-102")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// review-outcome.json: rework_implementation_gap with 1 failed manual check
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-102",
		"outcome":     "rework_implementation_gap",
		"summary":     "Keyboard navigation is broken in the modal component",
		"reviewed_at": "2026-03-16T14:15:00Z",
		"manual_results": []map[string]string{
			{"id": "check-a11y-keyboard", "result": "fail", "notes": "Keyboard nav broken"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	// process-review.json with trust level "medium"
	processReview := map[string]interface{}{
		"trust_level":        "medium",
		"automatic_proof":    "Tests passed but coverage incomplete",
		"machine_review":     "No blocking issues found",
		"recommended_posture": "manual_check_carefully",
		"degraded_flags":     []string{"incomplete_coverage"},
		"repair_cycles":      1,
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	// product-review.json
	productReview := map[string]interface{}{
		"run_id":        "run-102",
		"is_diagnostic": false,
		"summary":       "Implementation complete but accessibility gaps remain",
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	// validation.json
	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "28 of 30 tests passed, 2 skipped",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// === Invoke ===
	// Simulate distiller output (will be replaced by actual distiller call once
	// the reviewdistiller package lands on main).
	now := time.Now().UTC()
	result := reworkDistillResult{
		RunID:     "run-102",
		SpecID:    "spec-keyboard-nav",
		Outcome:   "rework_implementation_gap",
		ModelTier: "sonnet",
		Summary:   "Implementation gap detected: keyboard navigation not functional in modal component despite passing automated tests",
		CreatedAt: now,
		Metadata: map[string]string{
			"run_id":  "run-102",
			"spec_id": "spec-keyboard-nav",
			"model":   "sonnet",
		},
		Proposals: []reworkProposal{
			{
				ID:                  fmt.Sprintf("run-102-validation_gap-%d", 1),
				Type:                "validation_gap",
				Title:               "Add keyboard navigation integration tests for modal components",
				Content:             "Automated tests did not cover keyboard navigation flows; add integration tests that simulate Tab, Escape, and Enter key interactions within modals",
				Confidence:          0.95,
				ConfidenceRationale: "Manual check explicitly failed on keyboard nav — no automated equivalent exists",
				EvidenceReferences:  []string{"review-outcome.json", "check-a11y-keyboard", "process-review.json"},
			},
			{
				ID:                  fmt.Sprintf("run-102-doctrine_rule-%d", 2),
				Type:                "doctrine_rule",
				Title:               "Require a11y checks for all interactive UI components",
				Content:             "Any spec touching interactive UI must include accessibility validation as an acceptance criterion",
				Confidence:          0.88,
				ConfidenceRationale: "Failed manual check indicates systematic gap in acceptance criteria",
				EvidenceReferences:  []string{"review-outcome.json", "check-a11y-keyboard"},
			},
			{
				ID:                  fmt.Sprintf("run-102-planner_heuristic-%d", 3),
				Type:                "planner_heuristic",
				Title:               "Split UI tasks into visual and interaction sub-tasks",
				Content:             "When a task involves interactive UI, create separate sub-tasks for visual rendering and keyboard/screen-reader interaction to prevent interaction gaps",
				Confidence:          0.82,
				ConfidenceRationale: "Implementation focused on visual correctness but missed interaction layer",
				EvidenceReferences:  []string{"review-outcome.json", "check-a11y-keyboard", "validation.json"},
			},
			{
				ID:                  fmt.Sprintf("run-102-validation_gap-%d", 4),
				Type:                "validation_gap",
				Title:               "Add focus-trap validation for modal dialogs",
				Content:             "Modal components should trap focus within the dialog and return focus on close; add automated validation for this pattern",
				Confidence:          0.79,
				ConfidenceRationale: "Common a11y pattern missing from validation suite, related to keyboard nav failure",
				EvidenceReferences:  []string{"check-a11y-keyboard", "process-review.json"},
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
		if len(p.EvidenceReferences) > 0 {
			fmt.Fprintf(&mdBuf, "**Evidence:** %s\n\n", strings.Join(p.EvidenceReferences, ", "))
		}
	}
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if err := os.WriteFile(markdownPath, []byte(mdBuf.String()), 0o644); err != nil {
		t.Fatalf("write distillation-proposals.md: %v", err)
	}

	// === Assert ===

	// 1. distillation-proposals.json exists and is parseable
	rawJSON, err := os.ReadFile(proposalsPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.json: %v", err)
	}
	var parsed reworkDistillResult
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("parse distillation-proposals.json: %v", err)
	}

	// 2. run_id is "run-102"
	if parsed.RunID != "run-102" {
		t.Errorf("expected run_id 'run-102', got %q", parsed.RunID)
	}

	// 3. Outcome is "rework_implementation_gap"
	if parsed.Outcome != "rework_implementation_gap" {
		t.Errorf("expected outcome 'rework_implementation_gap', got %q", parsed.Outcome)
	}

	// 4. 3-5 proposals
	if len(parsed.Proposals) < 3 || len(parsed.Proposals) > 5 {
		t.Errorf("expected 3-5 proposals, got %d", len(parsed.Proposals))
	}

	// 5. At least one validation_gap, doctrine_rule, or planner_heuristic
	hasGuardrailType := false
	for _, p := range parsed.Proposals {
		if p.Type == "validation_gap" || p.Type == "doctrine_rule" || p.Type == "planner_heuristic" {
			hasGuardrailType = true
			break
		}
	}
	if !hasGuardrailType {
		types := make([]string, len(parsed.Proposals))
		for i, p := range parsed.Proposals {
			types[i] = p.Type
		}
		t.Errorf("expected at least one validation_gap, doctrine_rule, or planner_heuristic, got types: %v", types)
	}

	// 6. At least one proposal references the failed manual check item "check-a11y-keyboard"
	hasFailedCheckRef := false
	for _, p := range parsed.Proposals {
		for _, ref := range p.EvidenceReferences {
			if ref == "check-a11y-keyboard" {
				hasFailedCheckRef = true
				break
			}
		}
		if hasFailedCheckRef {
			break
		}
	}
	if !hasFailedCheckRef {
		t.Error("expected at least one proposal to reference the failed manual check item 'check-a11y-keyboard' in evidence_references")
	}

	// 7. At least one proposal references an evidence file in evidence_references
	hasEvidenceFileRef := false
	evidenceFiles := map[string]bool{
		"review-outcome.json": true,
		"process-review.json": true,
		"validation.json":     true,
		"product-review.json": true,
	}
	for _, p := range parsed.Proposals {
		for _, ref := range p.EvidenceReferences {
			if evidenceFiles[ref] {
				hasEvidenceFileRef = true
				break
			}
		}
		if hasEvidenceFileRef {
			break
		}
	}
	if !hasEvidenceFileRef {
		t.Error("expected at least one proposal to reference an evidence file in evidence_references")
	}

	// 8. Each proposal has required schema fields populated
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
		if len(p.EvidenceReferences) == 0 {
			t.Errorf("proposal[%d]: expected non-empty EvidenceReferences", i)
		}
	}

	// 9. evidence_references field present in raw JSON
	jsonStr := string(rawJSON)
	if !strings.Contains(jsonStr, "evidence_references") {
		t.Error("expected evidence_references field in JSON output")
	}

	// 10. model_tier is populated
	if parsed.ModelTier == "" {
		t.Error("expected non-empty model_tier")
	}

	// 11. CreatedAt is non-zero
	if parsed.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	// 12. distillation-proposals.md exists and contains outcome
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}
	if !strings.Contains(string(mdData), "rework_implementation_gap") {
		t.Error("distillation-proposals.md should mention rework_implementation_gap outcome")
	}

	// 13. Markdown references the failed check
	if !strings.Contains(string(mdData), "check-a11y-keyboard") {
		t.Error("distillation-proposals.md should reference the failed check item")
	}

	// 14. Markdown mentions keyboard nav content
	if !strings.Contains(string(mdData), "keyboard navigation") {
		t.Error("distillation-proposals.md should mention keyboard navigation")
	}
}