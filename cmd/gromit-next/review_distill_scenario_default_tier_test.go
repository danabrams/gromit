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

func TestScenario_DistillationUsesConfiguredDefaultTier(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-109",
		SpecID:                "spec-add-caching",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 19, 14, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 19, 14, 8, 0, 0, time.UTC),
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

	evidenceDir := store.RunEvidenceDir("run-109")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Seed project.json with distiller_tier: low
	projectDir := tmp
	projectCfg := map[string]interface{}{
		"repo_path":      "/tmp/fixture-app",
		"specs_dir":      "specs",
		"distiller_tier": "low",
	}
	writeJSON(t, filepath.Join(projectDir, "project.json"), projectCfg)

	// Verify project.json has distiller_tier set to low
	projRaw, err := os.ReadFile(filepath.Join(projectDir, "project.json"))
	if err != nil {
		t.Fatalf("read project.json: %v", err)
	}
	if !strings.Contains(string(projRaw), `"distiller_tier": "low"`) {
		t.Fatal("precondition: project.json should have distiller_tier set to low")
	}

	// Seed review-outcome.json (prerequisite for distillation)
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-109",
		"outcome":     "accepted",
		"summary":     "Caching implementation is correct and well-tested",
		"reviewed_at": "2026-03-19T14:15:00Z",
		"manual_results": []map[string]string{
			{"id": "check-cache-invalidation", "result": "pass"},
			{"id": "check-ttl-behavior", "result": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	// === Invoke ===
	// Load distiller_tier from project.json via loadConfigTier
	tier, err := loadConfigTier(filepath.Join(projectDir, "project.json"))
	if err != nil {
		t.Fatalf("loadConfigTier: %v", err)
	}
	// Verify loadConfigTier read the correct tier from project.json config
	if tier != reviewdistiller.TierLow {
		t.Errorf("loadConfigTier should read tier 'low' from project.json, got %q", tier)
	}

	// Create stub LLM completer
	completer := &testStubLLMCompleter{}

	// Create DistillerInputs
	inputs := &reviewdistiller.DistillerInputs{
		RunID:  "run-109",
		SpecID: "spec-add-caching",
	}

	// Load spec content
	runDir := store.RunDir("run-109")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir runDir: %v", err)
	}
	specContent := "# Spec: Add Caching\n\nImplement caching for the application.\n"
	if err := os.WriteFile(filepath.Join(runDir, "spec.md"), []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	inputs.SpecContent = specContent

	// Load review-outcome.json
	outcomeData, err := os.ReadFile(filepath.Join(evidenceDir, "review-outcome.json"))
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}
	inputs.ReviewOutcome = json.RawMessage(outcomeData)

	// Call Distill
	result, err := reviewdistiller.Distill(inputs, completer, tier)
	if err != nil {
		t.Fatalf("distill: %v", err)
	}

	// Write distillation-proposals.json
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	proposalsJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := os.WriteFile(proposalsPath, proposalsJSON, 0o644); err != nil {
		t.Fatalf("write distillation-proposals.json: %v", err)
	}

	// Render and write distillation-proposals.md
	markdown := renderDistillationMarkdown(result)
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
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

	// 2. run_id is "run-109"
	if parsed.RunID != "run-109" {
		t.Errorf("expected run_id 'run-109', got %q", parsed.RunID)
	}

	// 3. model_tier reflects "low" from project.json distiller_tier config
	if parsed.ModelTier != reviewdistiller.TierLow {
		t.Errorf("expected model_tier 'low' (from project.json distiller_tier), got %q", parsed.ModelTier)
	}

	// 4. model_tier is not the default "medium" (verifies config was read)
	jsonStr := string(rawJSON)
	if strings.Contains(jsonStr, `"model_tier": "medium"`) {
		t.Error("model_tier should be 'low' from config, not the default 'medium'")
	}

	// 5. Outcome is "accepted"
	if parsed.Outcome != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsed.Outcome)
	}

	// 6. CreatedAt is non-zero
	if parsed.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	// 7. Proposals are present
	if len(parsed.Proposals) < 1 {
		t.Errorf("expected at least 1 proposal, got %d", len(parsed.Proposals))
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
		if p.WhatHappened == "" {
			t.Errorf("proposal[%d]: expected non-empty WhatHappened", i)
		}
		if p.ProposedChange == "" {
			t.Errorf("proposal[%d]: expected non-empty ProposedChange", i)
		}
		if p.Confidence == "" {
			t.Errorf("proposal[%d]: expected non-empty Confidence", i)
		}
		if p.ConfidenceRationale == "" {
			t.Errorf("proposal[%d]: expected non-empty ConfidenceRationale", i)
		}
	}

	// 9. distillation-proposals.md exists and reflects low tier
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}

	mdStr := string(mdData)

	// 10. Markdown contains "low" model reference
	if !strings.Contains(mdStr, "low") {
		t.Error("distillation-proposals.md should reference low tier")
	}

	// 11. Markdown does not reference medium (the default tier)
	if strings.Contains(mdStr, "| medium") {
		t.Error("distillation-proposals.md should not reference default medium tier")
	}

	// 12. Markdown mentions the outcome
	if !strings.Contains(mdStr, "accepted") {
		t.Error("distillation-proposals.md should mention accepted outcome")
	}
}

// testStubLLMCompleter provides canned proposals for testing distillation.
type testStubLLMCompleter struct{}

func (s *testStubLLMCompleter) Complete(ctx context.Context, prompt string) (string, error) {
	return `[
    {
      "type": "doctrine_rule",
      "title": "Cache invalidation must pair with write operations",
      "what_happened": "Implementation added caching but invalidation patterns were incomplete",
      "what_was_missing": "Clear doctrine rule on cache invalidation strategies",
      "proposed_change": "Every write path that modifies cached data must include explicit cache invalidation",
      "rationale": "Prevents stale cache bugs in production",
      "confidence": "high",
      "confidence_rationale": "Pattern consistently observed in accepted caching implementations",
      "evidence_references": ["review-outcome.json"]
    },
    {
      "type": "planner_heuristic",
      "title": "Validate cache layer setup during planning",
      "what_happened": "Caching implementation passed manual review",
      "what_was_missing": "Early validation that cache infrastructure is properly initialized",
      "proposed_change": "Add a planner heuristic to validate cache setup during task decomposition",
      "rationale": "Catches infrastructure issues before implementation",
      "confidence": "high",
      "confidence_rationale": "Early validation prevents implementation delays",
      "evidence_references": ["acceptance.json"]
    }
  ]`, nil
}
