package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_ResumeOutcomePreservedDuringCleanup(t *testing.T) {
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	prior := runstore.NewRunState("spec-resume-outcome-preserved", "proj-resume-outcome")
	prior.Status = runstore.StatusReadyForReview
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(prior.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("prepare evidence dir: %v", err)
	}

	priorFinding := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityWarning,
		File:        "cleanup.go",
		Line:        12,
		Description: "cleanup evidence test",
		Disposition: review.DispositionNew,
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), map[string][]review.Finding{
		"spec_alignment": {priorFinding},
	})

	writeJSON(t, filepath.Join(evidenceDir, "manual-checklist.json"), map[string]string{"stale": "entry"})
	if err := os.WriteFile(filepath.Join(evidenceDir, "notes.txt"), []byte("temporary"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}
	cacheDir := filepath.Join(evidenceDir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "cache.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), map[string]string{"outcome": "rework"})

	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "review",
				fn: func(rs *runstore.RunState) {
					entries, err := os.ReadDir(evidenceDir)
					if err != nil {
						t.Fatalf("read evidence dir: %v", err)
					}
					if len(entries) != 1 || entries[0].Name() != "review-outcome.json" {
						t.Fatalf("expected only review-outcome.json after resume cleanup, got %+v", entries)
					}

					if _, err := os.Stat(filepath.Join(evidenceDir, "review.json")); !os.IsNotExist(err) {
						t.Fatalf("review.json should be removed before stages: %v", err)
					}
					if _, err := os.Stat(filepath.Join(evidenceDir, "manual-checklist.json")); !os.IsNotExist(err) {
						t.Fatalf("manual-checklist.json should be removed before stages: %v", err)
					}
					if _, err := os.Stat(filepath.Join(evidenceDir, "notes.txt")); !os.IsNotExist(err) {
						t.Fatalf("notes.txt should be removed before stages: %v", err)
					}
					if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
						t.Fatalf("cache directory should be removed before stages: %v", err)
					}

					if _, err := os.Stat(filepath.Join(evidenceDir, "review-outcome.json")); err != nil {
						t.Fatalf("review-outcome.json should remain: %v", err)
					}

					if len(rs.PriorReviewFindings) == 0 {
						t.Fatal("expected prior review findings to be populated")
					}
					if !strings.Contains(string(rs.PriorReviewFindings), "cleanup evidence test") {
						t.Fatalf("expected prior findings to mention our seeded data, got %s", string(rs.PriorReviewFindings))
					}
				},
			},
		},
	}

	run := &execSpecRun{
		specPath:      "spec.md",
		projectID:     "proj-resume-outcome",
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

	entries, err := os.ReadDir(evidenceDir)
	if err != nil {
		t.Fatalf("read evidence dir after run: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "review-outcome.json" {
		t.Fatalf("expected only review-outcome.json to remain after run, got %+v", entries)
	}
}

func TestExecScenarioResumeThrashStatePreserved(t *testing.T) {
	tmp := t.TempDir()

	store := runstore.NewStore(tmp)
	thrashFinding := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityError,
		File:        "thrash.go",
		Line:        42,
		Description: "thrash failure",
	}
	fp := thrashFingerprintForTest(thrashFinding)

	prior := runstore.NewRunState("spec-resume-thrash", "proj-resume-thrash")
	prior.Status = runstore.StatusNeedsHuman
	prior.EndedAt = time.Now()
	prior.ReviewThrashCounts = map[string]int{fp: 2}
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	var seen *runstore.RunState
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "plan",
				fn: func(rs *runstore.RunState) {
					seen = rs
				},
			},
		},
	}

	run := &execSpecRun{
		specPath:      "spec.md",
		projectID:     "proj-resume-thrash",
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

	if seen == nil {
		t.Fatal("expected stage to observe resumed RunState")
	}
	if count := seen.ReviewThrashCounts[fp]; count != 2 {
		t.Fatalf("expected thrash count 2 in resumed state, got %d", count)
	}

	loaded, err := store.Get(prior.RunID)
	if err != nil {
		t.Fatalf("load resumed run: %v", err)
	}
	if count := loaded.ReviewThrashCounts[fp]; count != 2 {
		t.Fatalf("expected persisted thrash count 2, got %d", count)
	}
}
