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

func TestScenario_EmptySuggestedFixUsesDescription(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	specsDir := filepath.Join(tmp, "specs")

	store := runstore.NewStore(storeDir)
	run := &runstore.RunState{
		RunID:     "run-empty-fix",
		SpecID:    "spec-empty-fix",
		ProjectID: "project-1",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	reviewJSON := map[string]interface{}{
		"performance": []interface{}{
			map[string]interface{}{
				"file":          "internal/scanner.go",
				"line":          42,
				"severity":      "warning",
				"facet":         "Regexp Compilation",
				"description":   "regexp compiled inside function body",
				"suggested_fix": "",
			},
		},
	}

	evidenceDir := store.RunEvidenceDir("run-empty-fix")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}
	reviewData, err := json.MarshalIndent(reviewJSON, "", "  ")
	if err != nil {
		t.Fatalf("marshal review.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), reviewData, 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Invoke
	specPath, err := maybeGenerateRemediationSpec("run-empty-fix", storeDir, specsDir)
	if err != nil {
		t.Fatalf("maybeGenerateRemediationSpec: %v", err)
	}
	if specPath == "" {
		t.Fatal("expected spec path, got empty string")
	}

	// Assert
	content, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(content)

	// The acceptance criterion must use the description text in file-path-prefixed format
	expectedCriterion := "1. [internal/scanner.go] regexp compiled inside function body"
	if !strings.Contains(spec, expectedCriterion) {
		t.Errorf("acceptance criterion should use description when suggested_fix is empty.\nExpected substring: %q\nGot:\n%s", expectedCriterion, spec)
	}
}
