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

// mockLLMCompleter implements reviewdistiller.LLMCompleter for testing.
type mockLLMCompleter struct {
	response string
}

func (m *mockLLMCompleter) Complete(ctx interface{}, prompt string) (string, error) {
	return m.response, nil
}

// distillProposal represents a single distillation proposal for test assertions.
// Used by scenario tests for backwards compatibility.
type distillProposal struct {
	ID                  string  `json:"id"`
	Type                string  `json:"type"`
	Title               string  `json:"title"`
	Content             string  `json:"content,omitempty"`
	Confidence          float64 `json:"confidence,omitempty"`
	ConfidenceRationale string  `json:"confidence_rationale,omitempty"`
}

// distillResult represents the distillation output for test assertions.
// Used by scenario tests for backwards compatibility.
type distillResult struct {
	RunID     string            `json:"run_id"`
	SpecID    string            `json:"spec_id"`
	Outcome   string            `json:"outcome"`
	ModelTier string            `json:"model_tier"`
	Summary   string            `json:"summary"`
	Proposals []distillProposal `json:"proposals"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
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
	// Create mock LLM that returns 4 proposals (mixture of types)
	llmResponse := `{
		"proposals": [
			{
				"type": "doctrine_rule",
				"title": "Enforce test-before-commit for validation packages",
				"what_happened": "Validation changes lacked test coverage initially",
				"what_was_missing": "Corresponding test updates",
				"proposed_change": "Require tests alongside all validation package changes",
				"rationale": "Prevents regression in validation logic",
				"confidence": "high",
				"confidence_rationale": "Pattern observed consistently across 3 accepted runs",
				"evidence_references": ["review.json"]
			},
			{
				"type": "planner_heuristic",
				"title": "Split large validation tasks into unit and integration phases",
				"what_happened": "Validation work was planned monolithically",
				"what_was_missing": "Separation of concerns in planning",
				"proposed_change": "Create separate unit and integration test tasks",
				"rationale": "Allows independent review and verification",
				"confidence": "high",
				"confidence_rationale": "Runs that split validation work had 40% fewer rework cycles",
				"evidence_references": ["review.json"]
			},
			{
				"type": "doctrine_rule",
				"title": "Require acceptance criteria mapping in evidence",
				"what_happened": "Task completion lacked explicit AC mapping",
				"what_was_missing": "Documentation of AC satisfaction",
				"proposed_change": "Each completed task should map to at least one acceptance criterion",
				"rationale": "Ensures requirements are fully met",
				"confidence": "high",
				"confidence_rationale": "Accepted runs with explicit mapping had zero rework rate",
				"evidence_references": ["validation.json"]
			},
			{
				"type": "validation_gap",
				"title": "Document edge case handling strategy",
				"what_happened": "Edge cases were handled implicitly",
				"what_was_missing": "Explicit documentation of edge case strategy",
				"proposed_change": "Add comments explaining edge case handling decisions",
				"rationale": "Improves future maintainability",
				"confidence": "medium",
				"confidence_rationale": "Based on comparison with similar spec complexity runs",
				"evidence_references": ["validation.json"]
			}
		]
	}`

	mockLLM := &mockLLMCompleter{response: llmResponse}

	// Load evidence files as json.RawMessage
	reviewOutcomeData, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}

	validationDataFile, err := os.ReadFile(filepath.Join(evidenceDir, "validation.json"))
	if err != nil {
		t.Fatalf("read validation.json: %v", err)
	}

	// Create DistillerInputs
	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-101",
		SpecID:        "spec-add-logging",
		SpecContent:   "Example spec for add-logging",
		ReviewOutcome: json.RawMessage(reviewOutcomeData),
		Validation:    json.RawMessage(validationDataFile),
	}

	// Call Distill
	result, err := reviewdistiller.Distill(inputs, mockLLM, reviewdistiller.TierHigh)
	if err != nil {
		t.Fatalf("Distill() returned error: %v", err)
	}

	// === Write JSON and Markdown outputs ===
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	proposalsJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := os.WriteFile(proposalsPath, proposalsJSON, 0o644); err != nil {
		t.Fatalf("write distillation-proposals.json: %v", err)
	}

	// Write distillation-proposals.md
	var mdBuf strings.Builder
	fmt.Fprintf(&mdBuf, "# Distillation Proposals\n\n")
	fmt.Fprintf(&mdBuf, "**Run:** %s | **Outcome:** %s | **Model:** %s\n\n", result.RunID, result.Outcome, string(result.ModelTier))
	fmt.Fprintf(&mdBuf, "## Proposals\n\n")
	for i, p := range result.Proposals {
		fmt.Fprintf(&mdBuf, "### %d. [%s] %s\n\n", i+1, p.Type, p.Title)
		fmt.Fprintf(&mdBuf, "**What Happened:** %s\n\n", p.WhatHappened)
		fmt.Fprintf(&mdBuf, "**What Was Missing:** %s\n\n", p.WhatWasMissing)
		fmt.Fprintf(&mdBuf, "**Proposed Change:** %s\n\n", p.ProposedChange)
		fmt.Fprintf(&mdBuf, "**Rationale:** %s\n\n", p.Rationale)
		fmt.Fprintf(&mdBuf, "**Confidence:** %s — %s\n\n", p.Confidence, p.ConfidenceRationale)
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
	var parsed reviewdistiller.DistillationResult
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

	// 9. Each proposal has all required schema fields populated
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
