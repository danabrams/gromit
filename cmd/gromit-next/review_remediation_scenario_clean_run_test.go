package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_AcceptedRunNoFindings_SkipsRemediation(t *testing.T) {
	// Seed: create a terminal run for spec "clean-feature" with review.json
	// containing only metadata (no finding arrays).
	tmpDir := t.TempDir()
	storeDir := filepath.Join(tmpDir, "store")
	specsDir := filepath.Join(tmpDir, "specs")

	run := &runstore.RunState{
		RunID:     "run-clean-001",
		SpecID:    "clean-feature",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now().Add(-1 * time.Minute),
		EndedAt:   time.Now(),
		Tasks:     []runstore.Task{{Status: "done"}},
	}

	store := runstore.NewStore(storeDir)
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Seed review.json with only metadata — no finding arrays
	reviewJSON := map[string]interface{}{
		"diff_unavailable": false,
	}

	evidenceDir := store.RunEvidenceDir("run-clean-001")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}

	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal review.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Invoke: call maybeGenerateRemediationSpec directly
	specPath, err := maybeGenerateRemediationSpec("run-clean-001", storeDir, specsDir)

	// Assert: no error
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec returned error: %v", err)
	}

	// Assert: empty path returned (no remediation spec generated)
	if specPath != "" {
		t.Errorf("expected empty spec path for clean run, got %q", specPath)
	}

	// Assert: no remediation spec file written in specsDir
	if entries, err := os.ReadDir(specsDir); err == nil && len(entries) > 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected no files in specsDir, found: %v", names)
	}

	// Assert: specsDir itself was never created (no MkdirAll called)
	if _, err := os.Stat(specsDir); err == nil {
		t.Error("specsDir should not exist when no remediation spec is generated")
	}
}
