package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestCleanupEvidenceDir_MissingDirectoryReturnsNil verifies that when the evidence
// directory does not exist, handleResumeEvidence (which calls cleanupEvidenceDir)
// returns nil — a missing directory is not an error.
func TestCleanupEvidenceDir_MissingDirectoryReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	rs := runstore.NewRunState("spec-missing-dir", "proj-missing-dir")
	if err := store.Save(rs); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Evidence directory is never created — it does not exist on disk.
	e := &execSpecRun{store: store}

	err := e.handleResumeEvidence(rs)

	if err != nil {
		t.Fatalf("expected nil for missing evidence dir, got: %v", err)
	}
	if len(rs.PriorReviewFindings) != 0 {
		t.Fatalf("expected PriorReviewFindings empty, got: %s", rs.PriorReviewFindings)
	}
}

// TestCleanupEvidenceDir_DirectlyNilOnMissingDir calls cleanupEvidenceDir directly
// with a path that does not exist, confirming it returns nil without wrapping the
// IsNotExist error.
func TestCleanupEvidenceDir_DirectlyNilOnMissingDir(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	e := &execSpecRun{store: store}

	nonexistent := filepath.Join(tmp, "does", "not", "exist")
	if err := e.cleanupEvidenceDir(nonexistent); err != nil {
		t.Fatalf("expected nil for nonexistent dir, got: %v", err)
	}
}

// Note: the os.IsNotExist branch inside the RemoveAll loop (lines 285-287 of exec.go)
// is dead code in practice: os.RemoveAll swallows IsNotExist internally and never
// returns it to the caller. The guard is harmless but untestable without mocking the
// OS layer, so no test is added for that branch.

func TestLoadPriorReviewFindings_NonNotExistErrorIsSurfaced(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("spec-dir-collision", "proj-dir-collision")
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(prior.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("prepare evidence dir: %v", err)
	}

	// Place a directory where review.json would be — os.ReadFile on a directory
	// returns a non-IsNotExist error.
	reviewJSONPath := filepath.Join(evidenceDir, "review.json")
	if err := os.Mkdir(reviewJSONPath, 0o755); err != nil {
		t.Fatalf("create directory at review.json path: %v", err)
	}

	var buf bytes.Buffer
	e := &execSpecRun{
		store: store,
		out:   &buf,
	}

	err := e.handleResumeEvidence(prior)

	if err != nil {
		t.Fatalf("expected nil error (fail-open), got: %v", err)
	}
	if len(prior.PriorReviewFindings) != 0 {
		t.Fatalf("expected PriorReviewFindings to be empty, got: %s", prior.PriorReviewFindings)
	}
	if !strings.Contains(buf.String(), "warning:") {
		t.Fatalf("expected warning to be written, got: %q", buf.String())
	}
}
