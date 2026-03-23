package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ReworkOutcome_DoesNotGenerateRemediation(t *testing.T) {
	// Seed
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}

	run := &runstore.RunState{
		RunID:     "run-rework-001",
		SpecID:    "spec-0042",
		ProjectID: "project-1",
		Status:    runstore.StatusReadyForReview,
		StartedAt: time.Now().Add(-5 * time.Minute),
		EndedAt:   time.Now(),
	}

	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed review packet artifacts so loadRunAndEnsurePacket returns early
	evidenceDir := store.RunEvidenceDir("run-rework-001")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	productReview := map[string]interface{}{
		"run_id":         "run-rework-001",
		"spec_title":     "spec-0042",
		"terminal_state": "ready_for_review",
		"summary":        "Completed with findings",
		"behavior_cards": []interface{}{},
		"is_diagnostic":  false,
	}
	processReview := map[string]interface{}{
		"trust_level":           "medium",
		"automatic_proof":       "pass",
		"machine_review":        "pass",
		"acceptance":            "pass",
		"repair_cycles":         0,
		"repeated_failure_flag": false,
		"recommended_posture":   "review",
	}
	manualChecklist := map[string]interface{}{
		"items": []interface{}{},
	}

	for name, data := range map[string]interface{}{
		"product-review.json":   productReview,
		"process-review.json":   processReview,
		"manual-checklist.json": manualChecklist,
	} {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, name), b, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Seed review.json with non-blocking findings
	reviewJSON := map[string]interface{}{
		"security": []interface{}{
			map[string]interface{}{
				"file":          "auth.go",
				"line":          25,
				"severity":      "warning",
				"facet":         "Missing Input Validation",
				"description":   "User input not validated",
				"suggested_fix": "Add input validation",
			},
		},
		"style": []interface{}{
			map[string]interface{}{
				"file":          "main.go",
				"line":          10,
				"severity":      "suggestion",
				"facet":         "Naming Convention",
				"description":   "Variable uses snake_case",
				"suggested_fix": "Rename to camelCase",
			},
		},
	}
	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal review.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Invoke via cobra command with rework_implementation_gap outcome
	cmd := newReviewRecordCmd()
	cmd.SetArgs([]string{
		"--run", "run-rework-001",
		"--outcome", "rework_implementation_gap",
		"--summary", "Needs fixes",
		"--store-dir", storeDir,
		"--specs-dir", specsDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	// Assert: no remediation spec was generated
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		t.Fatalf("read specs dir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() == "spec-0042-remediation.md" {
			t.Error("remediation spec should NOT be generated for rework_implementation_gap outcome")
		}
	}
	if len(entries) > 0 {
		t.Errorf("expected no files in specs dir, found %d", len(entries))
	}

	// Assert: review-outcome.json WAS written (the record itself should succeed)
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomePath); err != nil {
		t.Errorf("review-outcome.json should exist: %v", err)
	}

	// Assert: outcome file contains rework_implementation_gap
	outcomeData, err := os.ReadFile(outcomePath)
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}
	var outcomeMap map[string]interface{}
	if err := json.Unmarshal(outcomeData, &outcomeMap); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}
	if got, _ := outcomeMap["outcome"].(string); got != "rework_implementation_gap" {
		t.Errorf("expected outcome 'rework_implementation_gap', got %q", got)
	}
}

func TestScenario_ReworkVisionChange_DoesNotGenerateRemediation(t *testing.T) {
	// Seed
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("create specs dir: %v", err)
	}

	run := &runstore.RunState{
		RunID:     "run-rework-vision-001",
		SpecID:    "spec-0043",
		ProjectID: "project-1",
		Status:    runstore.StatusReadyForReview,
		StartedAt: time.Now().Add(-5 * time.Minute),
		EndedAt:   time.Now(),
	}

	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed review packet artifacts so loadRunAndEnsurePacket returns early
	evidenceDir := store.RunEvidenceDir("run-rework-vision-001")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	productReview := map[string]interface{}{
		"run_id":         "run-rework-vision-001",
		"spec_title":     "spec-0043",
		"terminal_state": "ready_for_review",
		"summary":        "Completed with findings",
		"behavior_cards": []interface{}{},
		"is_diagnostic":  false,
	}
	processReview := map[string]interface{}{
		"trust_level":           "medium",
		"automatic_proof":       "pass",
		"machine_review":        "pass",
		"acceptance":            "pass",
		"repair_cycles":         0,
		"repeated_failure_flag": false,
		"recommended_posture":   "review",
	}
	manualChecklist := map[string]interface{}{
		"items": []interface{}{},
	}

	for name, data := range map[string]interface{}{
		"product-review.json":   productReview,
		"process-review.json":   processReview,
		"manual-checklist.json": manualChecklist,
	} {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, name), b, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Seed review.json with non-blocking findings
	reviewJSON := map[string]interface{}{
		"security": []interface{}{
			map[string]interface{}{
				"file":          "auth.go",
				"line":          25,
				"severity":      "warning",
				"facet":         "Missing Input Validation",
				"description":   "User input not validated",
				"suggested_fix": "Add input validation",
			},
		},
		"style": []interface{}{
			map[string]interface{}{
				"file":          "main.go",
				"line":          10,
				"severity":      "suggestion",
				"facet":         "Naming Convention",
				"description":   "Variable uses snake_case",
				"suggested_fix": "Rename to camelCase",
			},
		},
	}
	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal review.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Invoke via cobra command with rework_vision_change outcome
	cmd := newReviewRecordCmd()
	cmd.SetArgs([]string{
		"--run", "run-rework-vision-001",
		"--outcome", "rework_vision_change",
		"--summary", "Vision needs revision",
		"--store-dir", storeDir,
		"--specs-dir", specsDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	// Assert: no remediation spec was generated
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		t.Fatalf("read specs dir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() == "spec-0043-remediation.md" {
			t.Error("remediation spec should NOT be generated for rework_vision_change outcome")
		}
	}
	if len(entries) > 0 {
		t.Errorf("expected no files in specs dir, found %d", len(entries))
	}

	// Assert: review-outcome.json WAS written (the record itself should succeed)
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomePath); err != nil {
		t.Errorf("review-outcome.json should exist: %v", err)
	}

	// Assert: outcome file contains rework_vision_change
	outcomeData, err := os.ReadFile(outcomePath)
	if err != nil {
		t.Fatalf("read review-outcome.json: %v", err)
	}
	var outcomeMap map[string]interface{}
	if err := json.Unmarshal(outcomeData, &outcomeMap); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}
	if got, _ := outcomeMap["outcome"].(string); got != "rework_vision_change" {
		t.Errorf("expected outcome 'rework_vision_change', got %q", got)
	}
}
