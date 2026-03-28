package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

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
