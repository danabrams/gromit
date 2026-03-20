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

func TestScenario_DistillerAcceptsFewerThanThreeProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-108",
		SpecID:                "spec-trivial",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 20, 9, 1, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "sonnet"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-108")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	reviewOutcome := map[string]interface{}{
		"run_id":      "run-108",
		"outcome":     "accepted",
		"summary":     "Trivially simple spec, accepted without issues",
		"reviewed_at": "2026-03-20T09:05:00Z",
		"manual_results": []map[string]string{
			{"id": "check-1", "result": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	reviewData := map[string]interface{}{
		"product_review": map[string]string{"summary": "Simple change, meets requirements"},
		"process_review": map[string]string{"summary": "Clean single-task run"},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), reviewData)

	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 5 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// === Invoke ===
	// Simulate distiller returning only 2 proposals (LLM found only 2 things
	// worth proposing for this trivially simple spec). The lower bound of 3
	// is advisory and must not be enforced.
	now := time.Now().UTC()
	result := distillResult{
		RunID:     "run-108",
		SpecID:    "spec-trivial",
		Outcome:   "accepted",
		ModelTier: "sonnet",
		Summary:   "Simple accepted run with minimal learnings to extract",
		CreatedAt: now,
		Metadata: map[string]string{
			"run_id":  "run-108",
			"spec_id": "spec-trivial",
			"model":   "sonnet",
		},
		Proposals: []distillProposal{
			{
				ID:                  fmt.Sprintf("run-108-doctrine_rule-%d", 1),
				Type:                "doctrine_rule",
				Title:               "Keep trivial specs single-task",
				Content:             "Specs with a single acceptance criterion should map to one task",
				Confidence:          0.80,
				ConfidenceRationale: "Consistent with prior simple runs that completed in one cycle",
			},
			{
				ID:                  fmt.Sprintf("run-108-info-%d", 2),
				Type:                "info",
				Title:               "Sonnet tier sufficient for trivial specs",
				Content:             "This trivially simple spec completed successfully with sonnet tier",
				Confidence:          0.75,
				ConfidenceRationale: "Run completed under budget and on first attempt",
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

	// 1. distillation-proposals.json exists and is parseable
	rawJSON, err := os.ReadFile(proposalsPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.json: %v", err)
	}
	var parsed distillResult
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("parse distillation-proposals.json: %v", err)
	}

	// 2. run_id matches
	if parsed.RunID != "run-108" {
		t.Errorf("expected run_id 'run-108', got %q", parsed.RunID)
	}

	// 3. Outcome is "accepted"
	if parsed.Outcome != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsed.Outcome)
	}

	// 4. Exactly 2 proposals — the lower bound of 3 is advisory, not enforced
	if len(parsed.Proposals) != 2 {
		t.Errorf("expected exactly 2 proposals (advisory lower bound not enforced), got %d", len(parsed.Proposals))
	}

	// 5. Both proposals have all required schema fields populated
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

	// 6. At least one doctrine_rule or planner_heuristic (reinforcement type)
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

	// 7. CreatedAt is non-zero
	if parsed.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	// 8. Metadata populated
	if parsed.Metadata["run_id"] != "run-108" {
		t.Errorf("expected metadata run_id 'run-108', got %q", parsed.Metadata["run_id"])
	}

	// 9. distillation-proposals.md exists and reflects 2 proposals
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	mdStr := string(mdData)
	if !strings.Contains(mdStr, "accepted") {
		t.Error("distillation-proposals.md should mention accepted outcome")
	}
	if !strings.Contains(mdStr, "Keep trivial specs single-task") {
		t.Error("distillation-proposals.md should contain first proposal title")
	}
	if !strings.Contains(mdStr, "Sonnet tier sufficient for trivial specs") {
		t.Error("distillation-proposals.md should contain second proposal title")
	}
	// Verify no third proposal section exists
	if strings.Contains(mdStr, "### 3.") {
		t.Error("distillation-proposals.md should not contain a third proposal section")
	}
}