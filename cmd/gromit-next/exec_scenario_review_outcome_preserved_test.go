package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_Resume_ReviewOutcomeJSONIsPreserved(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	prior := runstore.NewRunState("spec-review-outcome-preserved", "proj-review-outcome")
	prior.Status = runstore.StatusReadyForReview
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(prior.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence dir: %v", err)
	}

	writeJSON(t, filepath.Join(evidenceDir, "review.json"), map[string][]review.Finding{
		"spec_alignment": {
			{
				Facet:       "spec_alignment",
				Severity:    review.SeverityWarning,
				File:        "resume.go",
				Line:        10,
				Description: "seed finding that should be loaded as prior",
				Disposition: review.DispositionNew,
			},
		},
	})
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), map[string]string{"outcome": "rework"})

	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "review",
				fn: func(rs *runstore.RunState) {
					// Assert (before stages run): review.json is removed
					if _, err := os.Stat(filepath.Join(evidenceDir, "review.json")); !os.IsNotExist(err) {
						t.Fatalf("expected review.json to be removed before stages run, got: %v", err)
					}

					// Assert (before stages run): review-outcome.json is preserved
					if _, err := os.Stat(filepath.Join(evidenceDir, "review-outcome.json")); err != nil {
						t.Fatalf("expected review-outcome.json to exist after cleanup: %v", err)
					}

					entries, err := os.ReadDir(evidenceDir)
					if err != nil {
						t.Fatalf("read evidence dir: %v", err)
					}
					if len(entries) != 1 || entries[0].Name() != "review-outcome.json" {
						t.Fatalf("expected only review-outcome.json after cleanup, got %+v", entries)
					}

					// Prior findings should still be loaded into run state even though review.json was cleaned
					if !strings.Contains(string(rs.PriorReviewFindings), "seed finding that should be loaded as prior") {
						t.Fatalf("expected prior findings in run state, got: %s", string(rs.PriorReviewFindings))
					}
				},
			},
		},
	}

	// Invoke
	run := &execSpecRun{
		specPath:      "spec.md",
		projectID:     "proj-review-outcome",
		resumeRunID:   prior.RunID,
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}
	if err := run.run(context.Background()); err != nil {
		t.Fatalf("run resume command: %v", err)
	}

	// Assert
	if _, err := os.Stat(filepath.Join(evidenceDir, "review-outcome.json")); err != nil {
		t.Fatalf("expected review-outcome.json to still exist after run: %v", err)
	}
}
