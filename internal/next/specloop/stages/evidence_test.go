package stages

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// Verify EvidenceStage satisfies the Stage interface.
var _ specloop.Stage = (*EvidenceStage)(nil)

func TestEvidenceStage_AssemblesBundle(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-001", "proj-001")
	rs.Status = runstore.StatusReadyForReview
	rs.FinalValidationPassed = true
	rs.Tasks = []runstore.Task{
		{TaskID: "t-001", Status: "done", Objective: "first"},
		{TaskID: "t-002", Status: "done", Objective: "second"},
	}

	stage := NewEvidenceStage(store, EvidenceStageConfig{
		DiffSummary: "added 2 files",
	})

	if stage.Name() != "evidence" {
		t.Fatalf("expected name 'evidence', got %q", stage.Name())
	}

	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue, got %v", action.Kind)
	}

	evidenceDir := store.RunEvidenceDir(rs.RunID)

	expectedFiles := []string{
		"task-results.json",
		"validation.json",
		"metrics.json",
		"diff-summary.md",
		"summary.md",
		"review.md",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(evidenceDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected evidence file %s to exist", f)
		}
	}
}
