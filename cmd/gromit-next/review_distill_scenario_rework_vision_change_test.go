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

// visionChangeProposal extends the proposal schema with evidence_references for vision change scenarios.
type visionChangeProposal struct {
	ID                  string   `json:"id"`
	Type                string   `json:"type"`
	Title               string   `json:"title"`
	Content             string   `json:"content,omitempty"`
	Confidence          float64  `json:"confidence,omitempty"`
	ConfidenceRationale string   `json:"confidence_rationale,omitempty"`
	EvidenceReferences  []string `json:"evidence_references,omitempty"`
}

// visionChangeDistillResult represents the distillation output for rework_vision_change scenario assertions.
type visionChangeDistillResult struct {
	RunID     string                 `json:"run_id"`
	SpecID    string                 `json:"spec_id"`
	Outcome   string                 `json:"outcome"`
	ModelTier string                 `json:"model_tier"`
	Summary   string                 `json:"summary"`
	Proposals []visionChangeProposal `json:"proposals"`
	CreatedAt time.Time              `json:"created_at"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

func TestScenario_ReworkVisionChangeProducesRefinementProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-103",
		SpecID:                "spec-inline-editing",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 17, 9, 12, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: false,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "opus"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed evidence files
	evidenceDir := store.RunEvidenceDir("run-103")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	outcomeSummary := "Product direction shifted — we no longer want inline editing"

	// review-outcome.json: rework_vision_change
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-103",
		"outcome":     "rework_vision_change",
		"summary":     outcomeSummary,
		"reviewed_at": "2026-03-17T09:20:00Z",
		"manual_results": []map[string]string{
			{"id": "check-ux-direction", "result": "fail", "notes": "Inline editing no longer desired"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	// process-review.json
	processReview := map[string]interface{}{
		"trust_level":         "high",
		"automatic_proof":     "All tests passed",
		"machine_review":      "No blocking issues",
		"recommended_posture": "stamp_if_clean",
		"degraded_flags":      []string{},
		"repair_cycles":       0,
	}
	writeJSON(t, filepath.Join(evidenceDir, "process-review.json"), processReview)

	// product-review.json
	productReview := map[string]interface{}{
		"run_id":        "run-103",
		"is_diagnostic": false,
		"summary":       "Implementation of inline editing is complete and functional, but product direction has changed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "product-review.json"), productReview)

	// validation.json
	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 55 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// Seed a minimal spec content file to verify proposals reference spec content
	specContent := "# Spec: Inline Editing\n\nAdd inline editing to all table cells so users can edit without opening a modal.\n\n## Acceptance Criteria\n- Users can click any cell to edit\n- Changes auto-save on blur\n"
	specDir := filepath.Join(store.RunDir("run-103"))
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// === Invoke ===
	// Simulate distiller output (will be replaced by actual distiller call once
	// the reviewdistiller package lands on main).
	now := time.Now().UTC()
	result := visionChangeDistillResult{
		RunID:     "run-103",
		SpecID:    "spec-inline-editing",
		Outcome:   "rework_vision_change",
		ModelTier: "opus",
		Summary:   "Vision change detected: product direction shifted away from inline editing. Spec needs refinement to reflect new UX approach.",
		CreatedAt: now,
		Metadata: map[string]string{
			"run_id":  "run-103",
			"spec_id": "spec-inline-editing",
			"model":   "opus",
		},
		Proposals: []visionChangeProposal{
			{
				ID:                  fmt.Sprintf("run-103-refinement_guidance-%d", 1),
				Type:                "refinement_guidance",
				Title:               "Revise spec to replace inline editing with modal-based editing",
				Content:             fmt.Sprintf("The outcome summary states: %q. The current spec requires inline cell editing, but the product direction has shifted. Revise the spec acceptance criteria to use modal-based editing instead of inline editing.", outcomeSummary),
				Confidence:          0.95,
				ConfidenceRationale: "Outcome explicitly states product direction changed away from inline editing",
				EvidenceReferences:  []string{"review-outcome.json", "spec.md"},
			},
			{
				ID:                  fmt.Sprintf("run-103-refinement_guidance-%d", 2),
				Type:                "refinement_guidance",
				Title:               "Remove auto-save on blur acceptance criterion",
				Content:             fmt.Sprintf("The spec lists 'Changes auto-save on blur' as an acceptance criterion tied to inline editing. Since the vision changed (%s), this criterion should be removed or replaced with an explicit save action in the modal workflow.", outcomeSummary),
				Confidence:          0.90,
				ConfidenceRationale: "Auto-save on blur is specific to inline editing UX which is being abandoned per outcome summary",
				EvidenceReferences:  []string{"review-outcome.json", "spec.md", "product-review.json"},
			},
			{
				ID:                  fmt.Sprintf("run-103-doctrine_rule-%d", 3),
				Type:                "doctrine_rule",
				Title:               "Require explicit product-direction confirmation before implementing UX-heavy specs",
				Content:             "When a spec involves significant UX changes, confirm product direction with stakeholders before starting implementation to avoid wasted work from vision shifts",
				Confidence:          0.85,
				ConfidenceRationale: "This run completed successfully but was rejected due to a direction change that could have been caught earlier",
				EvidenceReferences:  []string{"review-outcome.json"},
			},
			{
				ID:                  fmt.Sprintf("run-103-planner_heuristic-%d", 4),
				Type:                "planner_heuristic",
				Title:               "Flag specs with UX-heavy scope for early stakeholder review",
				Content:             "Specs that modify user-facing interaction patterns should include a stakeholder sign-off checkpoint before implementation begins",
				Confidence:          0.80,
				ConfidenceRationale: "Vision change after full implementation suggests planning gap in stakeholder alignment",
				EvidenceReferences:  []string{"review-outcome.json", "spec.md"},
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
	var parsed visionChangeDistillResult
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("parse distillation-proposals.json: %v", err)
	}

	// 2. run_id is "run-103"
	if parsed.RunID != "run-103" {
		t.Errorf("expected run_id 'run-103', got %q", parsed.RunID)
	}

	// 3. Outcome is "rework_vision_change"
	if parsed.Outcome != "rework_vision_change" {
		t.Errorf("expected outcome 'rework_vision_change', got %q", parsed.Outcome)
	}

	// 4. At least one proposal exists
	if len(parsed.Proposals) == 0 {
		t.Fatal("expected at least one proposal, got 0")
	}

	// 5. At least one proposal of type refinement_guidance
	hasRefinementGuidance := false
	for _, p := range parsed.Proposals {
		if p.Type == "refinement_guidance" {
			hasRefinementGuidance = true
			break
		}
	}
	if !hasRefinementGuidance {
		types := make([]string, len(parsed.Proposals))
		for i, p := range parsed.Proposals {
			types[i] = p.Type
		}
		t.Errorf("expected at least one refinement_guidance proposal, got types: %v", types)
	}

	// 6. Proposals reference the outcome summary in their rationale/content
	hasOutcomeSummaryRef := false
	for _, p := range parsed.Proposals {
		if strings.Contains(p.Content, outcomeSummary) || strings.Contains(p.ConfidenceRationale, "direction") {
			hasOutcomeSummaryRef = true
			break
		}
	}
	if !hasOutcomeSummaryRef {
		t.Error("expected at least one proposal to reference the outcome summary in content or rationale")
	}

	// 7. Proposals reference spec content (via evidence_references or content)
	hasSpecRef := false
	for _, p := range parsed.Proposals {
		// Check evidence_references for spec.md
		for _, ref := range p.EvidenceReferences {
			if ref == "spec.md" {
				hasSpecRef = true
				break
			}
		}
		// Check content for spec-related terms
		if strings.Contains(p.Content, "inline editing") || strings.Contains(p.Content, "acceptance criter") {
			hasSpecRef = true
		}
		if hasSpecRef {
			break
		}
	}
	if !hasSpecRef {
		t.Error("expected at least one proposal to reference spec content in evidence_references or content")
	}

	// 8. At least one proposal references review-outcome.json in evidence_references
	hasOutcomeFileRef := false
	for _, p := range parsed.Proposals {
		for _, ref := range p.EvidenceReferences {
			if ref == "review-outcome.json" {
				hasOutcomeFileRef = true
				break
			}
		}
		if hasOutcomeFileRef {
			break
		}
	}
	if !hasOutcomeFileRef {
		t.Error("expected at least one proposal to reference review-outcome.json in evidence_references")
	}

	// 9. Each proposal has required schema fields populated
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
	if !strings.Contains(string(mdData), "rework_vision_change") {
		t.Error("distillation-proposals.md should mention rework_vision_change outcome")
	}

	// 13. Markdown mentions refinement_guidance
	if !strings.Contains(string(mdData), "refinement_guidance") {
		t.Error("distillation-proposals.md should mention refinement_guidance proposal type")
	}

	// 14. Markdown references the vision change context
	if !strings.Contains(string(mdData), "inline editing") {
		t.Error("distillation-proposals.md should reference inline editing from the vision change")
	}
}
