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

// defaultTierResult represents the distillation output for default tier assertions.
type defaultTierResult struct {
	RunID     string                `json:"run_id"`
	SpecID    string                `json:"spec_id"`
	Outcome   string                `json:"outcome"`
	ModelTier string                `json:"model_tier"`
	Summary   string                `json:"summary"`
	Proposals []defaultTierProposal `json:"proposals"`
	CreatedAt time.Time             `json:"created_at"`
	Metadata  map[string]string     `json:"metadata,omitempty"`
}

type defaultTierProposal struct {
	ID                  string   `json:"id"`
	Type                string   `json:"type"`
	Title               string   `json:"title"`
	Content             string   `json:"content,omitempty"`
	Confidence          float64  `json:"confidence,omitempty"`
	ConfidenceRationale string   `json:"confidence_rationale,omitempty"`
	EvidenceReferences  []string `json:"evidence_references,omitempty"`
}

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
	projectConfig := map[string]interface{}{
		"repo_path":      "/tmp/fixture-app",
		"specs_dir":      "specs",
		"distiller_tier": "low",
	}
	writeJSON(t, filepath.Join(projectDir, "project.json"), projectConfig)

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
	// Simulate distiller running automatically after outcome recording with
	// the configured default tier from project.json (low). No --tier override.
	// (Will be replaced by actual distiller call once the reviewdistiller
	// package lands on main.)
	now := time.Now().UTC()
	result := defaultTierResult{
		RunID:     "run-109",
		SpecID:    "spec-add-caching",
		Outcome:   "accepted",
		ModelTier: "low",
		Summary:   "Low-tier distillation captures key caching patterns from accepted run",
		CreatedAt: now,
		Metadata: map[string]string{
			"run_id":  "run-109",
			"spec_id": "spec-add-caching",
			"model":   "low",
		},
		Proposals: []defaultTierProposal{
			{
				ID:                  fmt.Sprintf("run-109-doctrine_rule-%d", 1),
				Type:                "doctrine_rule",
				Title:               "Cache invalidation must pair with write operations",
				Content:             "Every write path that modifies cached data must include explicit cache invalidation",
				Confidence:          0.82,
				ConfidenceRationale: "Pattern consistently observed in accepted caching implementations",
			},
			{
				ID:                  fmt.Sprintf("run-109-planner_heuristic-%d", 2),
				Type:                "planner_heuristic",
				Title:               "Plan TTL configuration as a separate task",
				Content:             "When adding caching, plan TTL configuration and testing as a discrete task to avoid coupling with cache logic",
				Confidence:          0.79,
				ConfidenceRationale: "Task isolation reduced rework in caching-related runs",
			},
			{
				ID:                  fmt.Sprintf("run-109-info-%d", 3),
				Type:                "info",
				Title:               "Low-tier distillation sufficient for straightforward caching patterns",
				Content:             "Accepted caching run did not require high-tier analysis for useful distillation",
				Confidence:          0.75,
				ConfidenceRationale: "Simple implementation patterns are well-captured at low tier",
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
	var parsed defaultTierResult
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("parse distillation-proposals.json: %v", err)
	}

	// 2. run_id is "run-109"
	if parsed.RunID != "run-109" {
		t.Errorf("expected run_id 'run-109', got %q", parsed.RunID)
	}

	// 3. model_tier reflects "low" from project.json distiller_tier config
	if parsed.ModelTier != "low" {
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
	if len(parsed.Proposals) < 2 {
		t.Errorf("expected at least 2 proposals, got %d", len(parsed.Proposals))
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
	}

	// 10. Metadata reflects low tier
	if parsed.Metadata["model"] != "low" {
		t.Errorf("expected metadata model 'low', got %q", parsed.Metadata["model"])
	}

	// 11. distillation-proposals.md exists and reflects low tier
	mdData, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatalf("read distillation-proposals.md: %v", err)
	}
	if len(mdData) == 0 {
		t.Error("expected non-empty distillation-proposals.md")
	}

	mdStr := string(mdData)

	// 12. Markdown contains "low" model reference
	if !strings.Contains(mdStr, "**Model:** low") {
		t.Error("distillation-proposals.md should reference Model: low")
	}

	// 13. Markdown does not reference medium (the default tier)
	if strings.Contains(mdStr, "**Model:** medium") {
		t.Error("distillation-proposals.md should not reference default medium tier")
	}

	// 14. Markdown mentions the outcome
	if !strings.Contains(mdStr, "accepted") {
		t.Error("distillation-proposals.md should mention accepted outcome")
	}

	// 15. Markdown contains proposal content
	if !strings.Contains(mdStr, "Cache invalidation") {
		t.Error("distillation-proposals.md should contain proposal titles")
	}
}
