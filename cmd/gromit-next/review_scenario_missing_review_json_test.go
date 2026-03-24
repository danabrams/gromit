package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ReviewRecord_MissingReviewJSON_AcceptSucceeds(t *testing.T) {
	// Seed: create a terminal run with review packet artifacts but no review.json
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	specsDir := filepath.Join(tmp, "specs")

	run := &runstore.RunState{
		RunID:     "run-no-review",
		SpecID:    "spec-0042",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now().Add(-time.Minute),
		EndedAt:   time.Now(),
	}

	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed review packet artifacts (required by loadRunAndEnsurePacket)
	// but deliberately do NOT create review.json
	evidenceDir := store.RunEvidenceDir("run-no-review")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	seedPacketArtifacts(t, evidenceDir)

	// Verify precondition: review.json does NOT exist
	if _, err := os.Stat(filepath.Join(evidenceDir, "review.json")); err == nil {
		t.Fatal("precondition: review.json should not exist in evidence directory")
	}

	// Invoke: reviewRecord should succeed despite missing review.json
	// (review.json is only consumed by maybeGenerateRemediationSpec, not by reviewRecord)
	err := reviewRecord("run-no-review", storeDir, "accepted", "Done", "")
	if err != nil {
		t.Fatalf("reviewRecord should succeed, got error: %v", err)
	}

	// Assert: review-outcome.json is written with correct content
	outcomeFile := filepath.Join(evidenceDir, "review-outcome.json")
	outcomeData, err := os.ReadFile(outcomeFile)
	if err != nil {
		t.Fatalf("review-outcome.json should exist: %v", err)
	}

	var outcome map[string]interface{}
	if err := json.Unmarshal(outcomeData, &outcome); err != nil {
		t.Fatalf("parse review-outcome.json: %v", err)
	}
	if outcome["outcome"] != "accepted" {
		t.Errorf("expected outcome 'accepted', got %v", outcome["outcome"])
	}
	if outcome["summary"] != "Done" {
		t.Errorf("expected summary 'Done', got %v", outcome["summary"])
	}

	// Invoke: maybeGenerateRemediationSpec should fail on missing review.json
	// (the cobra handler catches this error and writes a warning to stderr)
	specPath, specErr := maybeGenerateRemediationSpec("run-no-review", storeDir, specsDir)
	if specErr == nil {
		t.Fatal("maybeGenerateRemediationSpec should return error when review.json is missing")
	}
	if !strings.Contains(specErr.Error(), "review.json") {
		t.Errorf("error should mention review.json, got: %v", specErr)
	}

	// Assert: no remediation spec file is generated
	if specPath != "" {
		t.Errorf("expected empty spec path, got %q", specPath)
	}
	entries, err := os.ReadDir(specsDir)
	if err == nil && len(entries) > 0 {
		t.Errorf("expected no remediation spec files, found %d entries", len(entries))
	}
}

// seedPacketArtifacts writes minimal valid product-review.json,
// process-review.json, and manual-checklist.json into the given evidence directory.
func seedPacketArtifacts(t *testing.T, evidenceDir string) {
	t.Helper()

	artifacts := map[string]interface{}{
		"product-review.json": map[string]interface{}{
			"run_id":         "run-no-review",
			"spec_title":     "Test Spec",
			"terminal_state": "completed",
			"summary":        "All good",
			"behavior_cards": []interface{}{},
			"surprises":      []interface{}{},
			"is_diagnostic":  false,
		},
		"process-review.json": map[string]interface{}{
			"trust_level":           "high",
			"automatic_proof":       "passed",
			"machine_review":        "clean",
			"acceptance":            "all_criteria_met",
			"degraded_flags":        []interface{}{},
			"repair_cycles":         0,
			"repeated_failure_flag": false,
			"recommended_posture":   "accept",
		},
		"manual-checklist.json": map[string]interface{}{
			"items": []interface{}{},
		},
	}

	for name, data := range artifacts {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, name), b, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}
