package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// mockAuthLLMCompleter returns auth-specific proposals for testing tier overrides.
type mockAuthLLMCompleter struct{}

func (m *mockAuthLLMCompleter) Complete(ctx context.Context, prompt string) (string, error) {
	// Return mock proposals specific to auth refactoring
	mockResponse := `{
  "proposals": [
    {
      "type": "doctrine_rule",
      "title": "Enforce session token rotation on auth refactors",
      "what_happened": "Auth refactor completed without explicit session token rotation validation",
      "what_was_missing": "Documented requirement for token rotation as part of auth changes",
      "proposed_change": "Add doctrine rule requiring session token rotation validation in all auth refactors",
      "rationale": "Prevents stale session attacks and ensures security consistency",
      "confidence": "high",
      "confidence_rationale": "High-tier analysis identified this as a recurring security pattern",
      "evidence_references": ["review-outcome.json", "check-auth-flow"]
    },
    {
      "type": "planner_heuristic",
      "title": "Decompose auth tasks by middleware layer",
      "what_happened": "Auth refactor was split into multiple tasks but without clear layer boundaries",
      "what_was_missing": "Explicit decomposition strategy for auth tasks by middleware layers",
      "proposed_change": "Plan auth refactors with explicit tasks for: session management, token validation, and middleware wiring",
      "rationale": "Layer-based decomposition reduces rework and improves team coordination",
      "confidence": "high",
      "confidence_rationale": "High-tier analysis of task structure shows clear patterns",
      "evidence_references": ["review-outcome.json"]
    },
    {
      "type": "doctrine_rule",
      "title": "Require integration tests for auth middleware changes",
      "what_happened": "Auth changes passed validation with existing test suite",
      "what_was_missing": "Integration test coverage explicitly for middleware changes",
      "proposed_change": "Add doctrine rule requiring integration tests for any auth middleware modifications",
      "rationale": "Integration tests catch real-world auth flow issues that unit tests miss",
      "confidence": "high",
      "confidence_rationale": "High-tier cross-referencing of test coverage with acceptance criteria",
      "evidence_references": ["review-outcome.json", "check-session-mgmt"]
    }
  ]
}`
	return mockResponse, nil
}

func TestScenario_StandaloneDistillRerunsWithTierOverride(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-106",
		SpecID:                "spec-refactor-auth",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 18, 9, 12, 0, 0, time.UTC),
		FinalValidationPassed: true,
		FinalReviewPassed:     true,
		FinalAcceptancePassed: true,
		Tasks: []runstore.Task{
			{TaskID: "task-1", Status: "done", ModelTier: "sonnet"},
			{TaskID: "task-2", Status: "done", ModelTier: "sonnet"},
		},
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-106")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Seed review-outcome.json (prerequisite for distillation)
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-106",
		"outcome":     "accepted",
		"summary":     "Auth refactor looks good, all checks pass",
		"reviewed_at": "2026-03-18T09:20:00Z",
		"manual_results": []map[string]string{
			{"id": "check-auth-flow", "result": "pass"},
			{"id": "check-session-mgmt", "result": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	// Seed existing distillation-proposals.json from prior automatic distillation (sonnet tier)
	priorResult := reviewdistiller.DistillationResult{
		RunID:     "run-106",
		SpecID:    "spec-refactor-auth",
		Outcome:   "accepted",
		ModelTier: "sonnet",
		CreatedAt: time.Date(2026, 3, 18, 9, 25, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  "run-106-doctrine_rule-1",
				Type:                "doctrine_rule",
				Title:               "Old proposal from sonnet distillation",
				WhatHappened:        "Prior sonnet-tier distillation",
				Confidence:          "medium",
				ConfidenceRationale: "Lower confidence from sonnet tier",
			},
		},
	}
	priorJSON, err := json.MarshalIndent(priorResult, "", "  ")
	if err != nil {
		t.Fatalf("marshal prior result: %v", err)
	}
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	if err := os.WriteFile(proposalsPath, priorJSON, 0o644); err != nil {
		t.Fatalf("write prior distillation-proposals.json: %v", err)
	}

	// Seed existing distillation-proposals.md from prior automatic distillation
	priorMD := "# Distillation Proposals\n\n**Run:** run-106 | **Outcome:** accepted | **Model:** sonnet\n\n## Summary\n\nPrior automatic distillation with sonnet tier\n"
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	if err := os.WriteFile(markdownPath, []byte(priorMD), 0o644); err != nil {
		t.Fatalf("write prior distillation-proposals.md: %v", err)
	}

	// Verify prior files exist with sonnet tier
	priorRaw, err := os.ReadFile(proposalsPath)
	if err != nil {
		t.Fatalf("read prior proposals: %v", err)
	}
	if !strings.Contains(string(priorRaw), `"model_tier": "sonnet"`) {
		t.Fatal("precondition: prior proposals should have model_tier sonnet")
	}

	// === Invoke ===
	// Simulate `gromit-next review distill --run run-106 --tier high`
	// Re-run distillation with high tier override using reviewdistiller.Distill

	// Create spec.md in run directory
	runDir := store.RunDir("run-106")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	specContent := "# Refactor Auth\n\nRefactor the authentication system for improved security and maintainability.\n"
	specPath := filepath.Join(runDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// Build DistillerInputs from loaded artifacts
	outcomeData, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}

	inputs := &reviewdistiller.DistillerInputs{
		RunID:         "run-106",
		SpecID:        "spec-refactor-auth",
		SpecContent:   specContent,
		ReviewOutcome: json.RawMessage(outcomeData),
	}

	// Call reviewdistiller.Distill with TierHigh and mock completer
	llm := &mockAuthLLMCompleter{}
	result, err := reviewdistiller.Distill(inputs, llm, reviewdistiller.TierHigh)
	if err != nil {
		t.Fatalf("distill with tier high: %v", err)
	}

	// Write distillation-proposals.json with result
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := os.WriteFile(proposalsPath, resultJSON, 0o644); err != nil {
		t.Fatalf("write distillation-proposals.json: %v", err)
	}

	// Write distillation-proposals.md using renderDistillationMarkdown
	markdown := renderDistillationMarkdown(result)
	if err := os.WriteFile(markdownPath, []byte(markdown), 0o644); err != nil {
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

	// 2. run_id is "run-106"
	if parsed.RunID != "run-106" {
		t.Errorf("expected run_id 'run-106', got %q", parsed.RunID)
	}

	// 3. model_tier reflects "high" (the override), not "sonnet" (the prior tier)
	if parsed.ModelTier != reviewdistiller.TierHigh {
		t.Errorf("expected model_tier 'high' after tier override, got %q", parsed.ModelTier)
	}

	// 4. Prior sonnet tier is no longer present
	jsonStr := string(rawJSON)
	if strings.Contains(jsonStr, `"model_tier": "sonnet"`) {
		t.Error("distillation-proposals.json should not contain prior sonnet model_tier after override")
	}

	// 5. Outcome is still "accepted"
	if parsed.Outcome != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsed.Outcome)
	}

	// 6. CreatedAt is non-zero and newer than the prior distillation
	if parsed.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
	priorCreatedAt := time.Date(2026, 3, 18, 9, 25, 0, 0, time.UTC)
	if !parsed.CreatedAt.After(priorCreatedAt) {
		t.Errorf("expected created_at after prior distillation (%v), got %v", priorCreatedAt, parsed.CreatedAt)
	}

	// 7. New proposals are present (3 from mockAuthLLMCompleter)
	if len(parsed.Proposals) != 3 {
		t.Errorf("expected 3 proposals from high-tier distillation, got %d", len(parsed.Proposals))
	}

	// 8. Old sonnet proposal title is gone
	for _, p := range parsed.Proposals {
		if strings.Contains(p.Title, "Old proposal from sonnet") {
			t.Error("expected prior sonnet proposals to be overwritten, found old proposal title")
		}
	}

	// 9. At least one doctrine_rule or planner_heuristic in new proposals
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

	// 10. Each proposal has required schema fields populated
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
		if p.ConfidenceRationale == "" {
			t.Errorf("proposal[%d]: expected non-empty ConfidenceRationale", i)
		}
	}

	// 11. distillation-proposals.md was overwritten and reflects high tier
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}

	mdStr := string(mdData)

	// 12. Markdown contains "high" model reference, not "sonnet"
	if !strings.Contains(mdStr, "**Model Tier:** high") {
		t.Error("distillation-proposals.md should reference Model Tier: high")
	}
	if strings.Contains(mdStr, "**Model Tier:** sonnet") {
		t.Error("distillation-proposals.md should not reference prior sonnet model")
	}

	// 13. Markdown contains new proposal titles
	if !strings.Contains(mdStr, "session token rotation") {
		t.Error("distillation-proposals.md should contain new proposal content about session token rotation")
	}

	// 14. Markdown mentions the outcome
	if !strings.Contains(mdStr, "accepted") {
		t.Error("distillation-proposals.md should mention accepted outcome")
	}
}
