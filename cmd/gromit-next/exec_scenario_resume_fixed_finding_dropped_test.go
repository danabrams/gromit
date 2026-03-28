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

type scenarioCapturingReviewRunner struct {
	captured review.RunInput
}

func (c *scenarioCapturingReviewRunner) Run(_ context.Context, input review.RunInput) (*review.RunResult, error) {
	c.captured = input
	return &review.RunResult{FindingsByFacet: map[string][]review.Finding{}}, nil
}

func TestScenario_ResumeAfterRework_FixedFindingIsDropped(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)

	prior := runstore.NewRunState("spec-resume-rework-fixed-drop", "proj-resume-rework")
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
		File:        "promote.go",
		Line:        14,
		Description: "SourceRunID dead type",
		Disposition: review.DispositionNew,
	}
	writeJSON(t, filepath.Join(evidenceDir, "review.json"), map[string][]review.Finding{
		"spec_alignment": {priorFinding},
	})

	runner := &scenarioCapturingReviewRunner{}
	provider := &testStageProvider{
		stages: []specloop.Stage{
			&stageRecorderFunc{
				name: "review",
				fn: func(rs *runstore.RunState) {
					// Assert (at stage start): cleanup should remove stale evidence before stages run
					entries, err := os.ReadDir(evidenceDir)
					if err != nil {
						t.Fatalf("read evidence dir: %v", err)
					}
					if len(entries) != 0 {
						t.Fatalf("expected no artifacts after cleanup, got %+v", entries)
					}

					// Assert (at stage start): prior findings available before stage logic runs
					if len(rs.PriorReviewFindings) == 0 {
						t.Fatal("expected prior review findings to be populated")
					}
					if !strings.Contains(string(rs.PriorReviewFindings), "SourceRunID dead type") {
						t.Fatalf("expected prior findings to contain SourceRunID finding, got %s", string(rs.PriorReviewFindings))
					}

					var priorByFacet map[string][]review.Finding
					if err := json.Unmarshal(rs.PriorReviewFindings, &priorByFacet); err != nil {
						t.Fatalf("unmarshal prior findings: %v", err)
					}
					var priorList []review.Finding
					for _, findings := range priorByFacet {
						priorList = append(priorList, findings...)
					}

					// Invoke (review runner receives prior findings)
					if _, err := runner.Run(context.Background(), review.RunInput{
						SpecContent:   "spec content",
						DiffSummary:   "diff summary; rework removed type SourceRunID string from promote.go",
						Cycle:         rs.Cycle,
						PriorFindings: priorList,
					}); err != nil {
						t.Fatalf("run review runner: %v", err)
					}

					// Simulate new review result after fix: SourceRunID finding is gone
					writeJSON(t, filepath.Join(evidenceDir, "review.json"), map[string][]review.Finding{
						"spec_alignment": {
							{
								Facet:       "spec_alignment",
								Severity:    review.SeverityWarning,
								File:        "promote.go",
								Line:        20,
								Description: "promote.go now passes the new helper",
								Disposition: review.DispositionNew,
							},
						},
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

	// Assert (runner input)
	if len(runner.captured.PriorFindings) == 0 {
		t.Fatal("expected review runner to receive prior findings")
	}
	if !strings.Contains(runner.captured.PriorFindings[0].Description, "SourceRunID") {
		t.Fatalf("prior finding missing SourceRunID: %+v", runner.captured.PriorFindings)
	}

	// Assert (new review.json drops fixed finding)
	data, err := os.ReadFile(filepath.Join(evidenceDir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}
	if strings.Contains(string(data), "SourceRunID") {
		t.Fatalf("expected new review.json to drop SourceRunID finding, got %s", string(data))
	}
}
