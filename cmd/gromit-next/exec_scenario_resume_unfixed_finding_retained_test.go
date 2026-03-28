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
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), map[string][]review.Finding{
		"spec_alignment": {priorFinding},
	})
	writeJSON(t, filepath.Join(evidenceDir, "review-outcome.json"), map[string]string{"outcome": "rework"})

	agent := &retainingReviewAgent{
		findings: []review.Finding{
			{
				Severity:     review.SeverityWarning,
				File:         "rejection_history.go",
				Line:         55,
				Description:  "os.IsNotExist guard missing",
				SuggestedFix: "Wrap the os.Stat call in a guard for os.IsNotExist",
			},
		},
	}
	runner := review.NewRunner(agent, review.RunnerConfig{
		Facets:    []string{"spec_alignment"},
		Threshold: review.SeverityWarning,
	})

	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "review",
				fn: func(rs *runstore.RunState) {
					if len(rs.PriorReviewFindings) == 0 {
						t.Fatal("expected prior review findings to be populated")
					}
					if !strings.Contains(string(rs.PriorReviewFindings), "os.IsNotExist") {
						t.Fatalf("expected prior findings to mention os.IsNotExist, got %s", string(rs.PriorReviewFindings))
					}

					prior := loadPriorFindings(t, rs.PriorReviewFindings)
					result, err := runner.Run(context.Background(), review.RunInput{
						SpecContent:   "spec content",
						DiffSummary:   "diff summary",
						Cycle:         2,
						PriorFindings: prior,
					})
					if err != nil {
						t.Fatalf("run review runner: %v", err)
					}

					facetFindings := result.FindingsByFacet["spec_alignment"]
					if len(facetFindings) == 0 {
						t.Fatalf("expected spec_alignment finding, got %+v", result.FindingsByFacet)
					}
					if facetFindings[0].Disposition != review.DispositionPreExisting {
						t.Fatalf("expected disposition pre-existing, got %q", facetFindings[0].Disposition)
					}

					writeJSON(t, filepath.Join(evidenceDir, "review.json"), result.FindingsByFacet)
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
	var facets map[string][]review.Finding
	if err := json.Unmarshal(data, &facets); err != nil {
		t.Fatalf("unmarshal review.json: %v", err)
	}
	if len(facets) != 1 {
		t.Fatalf("expected exactly 1 facet key, got %d: %v", len(facets), facets)
	}
	findings, ok := facets["spec_alignment"]
	if !ok {
		t.Fatalf("expected facet key %q, got keys: %v", "spec_alignment", facets)
	}
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %v", len(findings), findings)
	}
	f := findings[0]
	if f.File != "rejection_history.go" {
		t.Fatalf("expected File %q, got %q", "rejection_history.go", f.File)
	}
	if f.Line != 55 {
		t.Fatalf("expected Line 55, got %d", f.Line)
	}
	if f.Description != "os.IsNotExist guard missing" {
		t.Fatalf("expected Description %q, got %q", "os.IsNotExist guard missing", f.Description)
	}
	if f.Disposition != review.DispositionPreExisting {
		t.Fatalf("expected Disposition %q, got %q", review.DispositionPreExisting, f.Disposition)
	}
}

func loadPriorFindings(t *testing.T, data json.RawMessage) []review.Finding {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal prior findings: %v", err)
	}

	var findings []review.Finding
	for facet, payload := range raw {
		if facet == "diff_unavailable" {
			continue
		}
		var facetFindings []review.Finding
		if err := json.Unmarshal(payload, &facetFindings); err != nil {
			t.Fatalf("unmarshal findings for %s: %v", facet, err)
		}
		findings = append(findings, facetFindings...)
	}
	return findings
}

type retainingReviewAgent struct {
	findings []review.Finding
}

func (a *retainingReviewAgent) ReviewFacet(_ context.Context, _ string, _ string) ([]review.Finding, error) {
	return a.findings, nil
}
