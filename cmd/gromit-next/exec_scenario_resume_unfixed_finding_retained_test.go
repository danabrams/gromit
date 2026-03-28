package main

import (
	"context"
	"encoding/json"
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

func TestScenario_ResumeAfterRework_UnfixedFindingIsRetainedAsPreExisting(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	prior := runstore.NewRunState("spec-resume-rework", "proj-resume-rework")
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
		File:        "rejection_history.go",
		Line:        55,
		Description: "os.IsNotExist guard missing",
		Disposition: review.DispositionNew,
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), []review.Finding{priorFinding})
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), map[string]string{"outcome": "rework"})

	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "review",
				fn: func(_ *runstore.RunState) {
					data, err := os.ReadFile(filepath.Join(evidenceDir, "review.json"))
					if err != nil {
						t.Fatalf("read seeded review.json: %v", err)
					}
					var priorLoaded []review.Finding
					if err := json.Unmarshal(data, &priorLoaded); err != nil {
						t.Fatalf("unmarshal prior findings: %v", err)
					}
					if len(priorLoaded) != 1 {
						t.Fatalf("expected 1 prior finding, got %d", len(priorLoaded))
					}
					if !strings.Contains(priorLoaded[0].Description, "os.IsNotExist") {
						t.Fatalf("expected prior finding description to mention os.IsNotExist, got %q", priorLoaded[0].Description)
					}

					retained := priorLoaded[0]
					retained.Disposition = review.DispositionPreExisting
					writeJSON(t, filepath.Join(evidenceDir, "review.json"), map[string][]review.Finding{
						"spec_alignment": {retained},
					})
				},
			},
		},
	}

	// Invoke
	run := &execSpecRun{
		specPath:      "spec.md",
		projectID:     "proj-resume-rework",
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
	data, err := os.ReadFile(filepath.Join(evidenceDir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "os.IsNotExist") {
		t.Fatalf("expected review.json to retain os.IsNotExist finding, got %s", content)
	}
	if !strings.Contains(content, `"disposition": "pre-existing"`) {
		t.Fatalf("expected review.json to mark finding as pre-existing, got %s", content)
	}
}
