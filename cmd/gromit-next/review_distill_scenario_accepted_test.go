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

// distillProposal represents a single distillation proposal for test assertions.
type distillProposal struct {
	ID                  string  `json:"id"`
	Type                string  `json:"type"`
	Title               string  `json:"title"`
	Content             string  `json:"content,omitempty"`
	Confidence          float64 `json:"confidence,omitempty"`
	ConfidenceRationale string  `json:"confidence_rationale,omitempty"`
}

// distillResult represents the distillation output for test assertions.
type distillResult struct {
	RunID     string             `json:"run_id"`
	SpecID    string             `json:"spec_id"`
	Outcome   string             `json:"outcome"`
	ModelTier string             `json:"model_tier"`
	Summary   string             `json:"summary"`
	Proposals []distillProposal  `json:"proposals"`
	CreatedAt time.Time          `json:"created_at"`
	Metadata  map[string]string  `json:"metadata,omitempty"`
}

func TestScenario_AcceptedRunProducesReinforcementProposals(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-101",
		SpecID:                "spec-add-logging",
		ProjectID:             "fixture-calc",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 15, 10, 5, 0, 0, time.UTC),
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

	// Seed evidence files
	evidenceDir := store.RunEvidenceDir("run-101")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	reviewOutcome := map[string]interface{}{
		"run_id":      "run-101",
		"outcome":     "accepted",
		"summary":     "All checks passed, implementation is solid",
		"reviewed_at": "2026-03-15T10:10:00Z",
		"manual_results": []map[string]string{
			{"id": "check-1", "result": "pass"},
			{"id": "check-2", "result": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	reviewData := map[string]interface{}{
		"product_review": map[string]string{"summary": "Implementation meets all requirements"},
		"process_review": map[string]string{"summary": "Clean process, good test coverage"},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), reviewData)

	validationData := map[string]interface{}{
		"pass":         true,
		"build_errors": []string{},
		"test_results": "All 42 tests passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// === Invoke ===
	// Simulate distiller output (will be replaced by actual distiller call once
	// the reviewdistiller package lands on main).
	now := time.Now().UTC()
	result := distillResult{
		RunID:     "run-101",
		SpecID:    "spec-add-logging",
		Outcome:   "accepted",
		ModelTier: "opus",
		Summary:   "Accepted run demonstrates strong implementation patterns worth reinforcing",
		CreatedAt: now,
		Metadata: map[string]string{
			"run_id":  "run-101",
			"spec_id": "spec-add-logging",
			"model":   "opus",
		},
		Proposals: []distillProposal{
			{
				ID:                  fmt.Sprintf("run-101-doctrine_rule-%d", 1),
				Type:                "doctrine_rule",
				Title:               "Enforce test-before-commit for validation packages",
				Content:             "All validation package changes should include corresponding test updates",
				Confidence:          0.92,
				ConfidenceRationale: "Pattern observed consistently across 3 accepted runs",
			},
			{
				ID:                  fmt.Sprintf("run-101-planner_heuristic-%d", 2),
				Type:                "planner_heuristic",
				Title:               "Split large validation tasks into unit and integration phases",
				Content:             "When a task touches validation logic, plan separate unit and integration test tasks",
				Confidence:          0.85,
				ConfidenceRationale: "Runs that split validation work had 40% fewer rework cycles",
			},
			{
				ID:                  fmt.Sprintf("run-101-info-%d", 3),
				Type:                "info",
				Title:               "Cost efficiency observation",
				Content:             "Run completed under budget with opus tier",
				Confidence:          0.78,
				ConfidenceRationale: "Based on comparison with similar spec complexity runs",
			},
			{
				ID:                  fmt.Sprintf("run-101-doctrine_rule-%d", 4),
				Type:                "doctrine_rule",
				Title:               "Require acceptance criteria mapping in evidence",
				Content:             "Each completed task should map to at least one acceptance criterion",
				Confidence:          0.88,
				ConfidenceRationale: "Accepted runs with explicit mapping had zero rework rate",
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

	// 2. run_id is "run-101"
	if parsed.RunID != "run-101" {
		t.Errorf("expected run_id 'run-101', got %q", parsed.RunID)
	}

	// 3. Non-empty spec_id
	if parsed.SpecID == "" {
		t.Error("expected non-empty spec_id")
	}

	// 4. Outcome is "accepted"
	if parsed.Outcome != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsed.Outcome)
	}

	// 5. model_tier is populated
	if parsed.ModelTier == "" {
		t.Error("expected non-empty model_tier")
	}

	// 6. CreatedAt is non-zero
	if parsed.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
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

	// 10. confidence_rationale present in raw JSON
	jsonStr := string(rawJSON)
	if !strings.Contains(jsonStr, "confidence_rationale") {
		t.Error("expected confidence_rationale field in JSON output")
	}

	// 11. distillation-proposals.md exists and is non-empty
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}

	// 12. Markdown contains outcome
	if !strings.Contains(string(mdData), "accepted") {
		t.Error("distillation-proposals.md should mention accepted outcome")
	}

	// 13. Markdown mentions at least one proposal title
	if !strings.Contains(string(mdData), "Enforce test-before-commit") {
		t.Error("distillation-proposals.md should contain proposal titles")
	}
}

func writeJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON for %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}