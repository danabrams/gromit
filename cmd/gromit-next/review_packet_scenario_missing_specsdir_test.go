package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_ReviewRecord_MissingSpecsDir_WarnsButSucceeds(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	store := runstore.NewStore(storeDir)

	run := &runstore.RunState{
		RunID:     "run-missing-specsdir",
		SpecID:    "spec-test",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now().Add(-1 * time.Minute),
		EndedAt:   time.Now(),
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed review packet artifacts so loadRunAndEnsurePacket succeeds
	evidenceDir := store.RunEvidenceDir("run-missing-specsdir")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	productReview := reviewpacket.ProductReview{
		RunID:         "run-missing-specsdir",
		SpecTitle:     "spec-test",
		TerminalState: "completed",
		Summary:       "All good",
		BehaviorCards: []reviewpacket.BehaviorCard{},
		Surprises:     []string{},
	}
	processReview := reviewpacket.ProcessReview{
		TrustLevel:         "high",
		AutomaticProof:     "pass",
		MachineReview:      "pass",
		Acceptance:         "pass",
		DegradedFlags:      []string{},
		RecommendedPosture: "accept",
	}
	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{},
	}

	for name, v := range map[string]interface{}{
		"product-review.json":   productReview,
		"process-review.json":   processReview,
		"manual-checklist.json": manualChecklist,
	} {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Also seed review.json with findings (to exercise the remediation path)
	reviewJSON := map[string]interface{}{
		"style": []interface{}{
			map[string]interface{}{
				"file":        "main.go",
				"line":        10,
				"severity":    "warning",
				"facet":       "Naming",
				"description": "Use camelCase",
			},
		},
	}
	reviewData, _ := json.MarshalIndent(reviewJSON, "", "  ")
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Invoke via cobra to capture stderr
	cmd := newReviewRecordCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"--run", "run-missing-specsdir",
		"--outcome", "accepted",
		"--summary", "Done",
		"--store-dir", storeDir,
		// No --specs-dir, no --project
	})

	err := cmd.Execute()

	// Assert: command succeeds
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Assert: review-outcome.json is written
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomePath); err != nil {
		t.Errorf("review-outcome.json not found: %v", err)
	}

	// Assert: stderr contains warning about skipping remediation spec generation
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "skipping remediation spec generation") {
		t.Errorf("expected stderr warning about skipping remediation spec generation, got:\n%s", stderrStr)
	}
	if !strings.Contains(stderrStr, "specs-dir not configured") {
		t.Errorf("expected stderr to mention specs-dir not configured, got:\n%s", stderrStr)
	}
}
