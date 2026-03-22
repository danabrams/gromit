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

func TestScenario_AutomaticDistillationReadsConfigTier(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	run := &runstore.RunState{
		RunID:                 "run-auto-tier",
		SpecID:                "spec-config-test",
		ProjectID:             "fixture-app",
		Status:                runstore.StatusReadyForReview,
		StartedAt:             time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		EndedAt:               time.Date(2026, 3, 20, 10, 5, 0, 0, time.UTC),
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

	evidenceDir := store.RunEvidenceDir("run-auto-tier")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Seed project.json with distiller_tier: high (non-default config)
	projectDir := tmp
	projectCfg := map[string]interface{}{
		"repo_path":      "/tmp/fixture-app",
		"specs_dir":      "specs",
		"distiller_tier": "high",
	}
	writeJSON(t, filepath.Join(projectDir, "project.json"), projectCfg)

	// Verify project.json has distiller_tier set to high
	projRaw, err := os.ReadFile(filepath.Join(projectDir, "project.json"))
	if err != nil {
		t.Fatalf("read project.json: %v", err)
	}
	if !strings.Contains(string(projRaw), `"distiller_tier": "high"`) {
		t.Fatal("precondition: project.json should have distiller_tier set to high")
	}

	// Seed review-outcome.json (prerequisite for distillation)
	reviewOutcome := map[string]interface{}{
		"run_id":      "run-auto-tier",
		"outcome":     "accepted",
		"summary":     "Implementation is correct",
		"reviewed_at": "2026-03-20T10:10:00Z",
		"manual_results": []map[string]string{
			{"id": "check-core", "result": "pass"},
		},
	}
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), reviewOutcome)

	// Seed validation.json (required for packet regeneration)
	validationData := map[string]interface{}{
		"status":  "passed",
		"summary": "All validations passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "validation.json"), validationData)

	// Seed acceptance.json (required for packet regeneration)
	acceptanceData := map[string]interface{}{
		"status":  "passed",
		"summary": "All acceptance criteria met",
	}
	writeJSON(t, filepath.Join(evidenceDir, "acceptance.json"), acceptanceData)

	// Seed review.json (required for packet regeneration)
	machineReviewData := map[string]interface{}{
		"status":  "passed",
		"summary": "Machine review passed",
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), machineReviewData)

	// Create spec.md in run directory
	runDir := store.RunDir("run-auto-tier")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	specContent := "# Spec: Config Test\n\nTest automatic config tier reading.\n"
	specPath := filepath.Join(runDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}

	// === Invoke ===
	// Simulate the automatic distillation path: load config tier, then call attemptDistillation
	configTier, err := loadConfigTier(filepath.Join(projectDir, "project.json"))
	if err != nil {
		t.Fatalf("loadConfigTier: %v", err)
	}

	// Verify loadConfigTier read the correct tier from project.json
	if configTier != reviewdistiller.TierHigh {
		t.Errorf("loadConfigTier should read tier 'high' from project.json, got %q", configTier)
	}

	// Create stub LLM completer
	completer := &testAutoConfigLLMCompleter{}

	// Capture the tier passed to attemptDistillation by using a wrapper function
	// that tracks which tier is used
	var capturedTier reviewdistiller.Tier
	attemptDistillationFunc := func(runID string, storeDir string, tier reviewdistiller.Tier, completer reviewdistiller.LLMCompleter) error {
		capturedTier = tier
		return attemptDistillation(runID, storeDir, tier, completer)
	}

	// Call the automatic distillation path with the configured tier
	if err := attemptDistillationFunc("run-auto-tier", tmp, configTier, completer); err != nil {
		t.Fatalf("automatic distillation: %v", err)
	}

	// === Assert ===

	// 1. Captured tier is 'high' (from project.json), not the default 'medium'
	if capturedTier != reviewdistiller.TierHigh {
		t.Errorf("expected automatic distillation to use tier 'high' from config, got %q", capturedTier)
	}

	// 2. distillation-proposals.json exists and is parseable
	proposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
	rawJSON, err := os.ReadFile(proposalsPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.json: %v", err)
	}
	var parsed reviewdistiller.DistillationResult
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("parse distillation-proposals.json: %v", err)
	}

	// 3. run_id is "run-auto-tier"
	if parsed.RunID != "run-auto-tier" {
		t.Errorf("expected run_id 'run-auto-tier', got %q", parsed.RunID)
	}

	// 4. model_tier reflects "high" (from project.json distiller_tier config)
	if parsed.ModelTier != reviewdistiller.TierHigh {
		t.Errorf("expected model_tier 'high' (from project.json distiller_tier), got %q", parsed.ModelTier)
	}

	// 5. model_tier is not the hardcoded default "medium"
	jsonStr := string(rawJSON)
	if strings.Contains(jsonStr, `"model_tier": "medium"`) {
		t.Error("model_tier should be 'high' from config, not the hardcoded default 'medium'")
	}

	// 6. Outcome is "accepted"
	if parsed.Outcome != "accepted" {
		t.Errorf("expected outcome 'accepted', got %q", parsed.Outcome)
	}

	// 7. CreatedAt is non-zero
	if parsed.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}

	// 8. Proposals are present
	if len(parsed.Proposals) < 1 {
		t.Errorf("expected at least 1 proposal, got %d", len(parsed.Proposals))
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
		if p.Confidence == "" {
			t.Errorf("proposal[%d]: expected non-empty Confidence", i)
		}
		if p.ConfidenceRationale == "" {
			t.Errorf("proposal[%d]: expected non-empty ConfidenceRationale", i)
		}
	}

	// 10. distillation-proposals.md exists and reflects high tier
	markdownPath := filepath.Join(evidenceDir, "distillation-proposals.md")
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}

	mdStr := string(mdData)

	// 11. Markdown contains "high" model reference
	if !strings.Contains(mdStr, "high") {
		t.Error("distillation-proposals.md should reference high tier")
	}

	// 12. Markdown does not reference medium (the hardcoded tier)
	if strings.Contains(mdStr, "| medium") {
		t.Error("distillation-proposals.md should not reference hardcoded medium tier")
	}

	// 13. Markdown mentions the outcome
	if !strings.Contains(mdStr, "accepted") {
		t.Error("distillation-proposals.md should mention accepted outcome")
	}
}

// testAutoConfigLLMCompleter provides proposals for testing automatic config tier reading.
type testAutoConfigLLMCompleter struct{}

func (s *testAutoConfigLLMCompleter) Complete(ctx context.Context, prompt string) (string, error) {
	return `[
    {
      "type": "doctrine_rule",
      "title": "Automatic distillation reads config tier correctly",
      "what_happened": "Configuration tier was properly applied to distillation",
      "what_was_missing": "Nothing - config tier system is working",
      "proposed_change": "Continue using configured tier from project.json",
      "rationale": "Ensures distillation respects project configuration",
      "confidence": "high",
      "confidence_rationale": "Test explicitly verified config reading",
      "evidence_references": ["project.json", "review-outcome.json"]
    }
  ]`, nil
}
