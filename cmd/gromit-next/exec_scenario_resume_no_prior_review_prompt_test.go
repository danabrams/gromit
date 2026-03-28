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

type promptCapturingReviewAgent struct {
	prompt string
}

func (a *promptCapturingReviewAgent) ReviewFacet(_ context.Context, _ string, prompt string) ([]review.Finding, error) {
	a.prompt = prompt
	return []review.Finding{}, nil
}

func TestScenario_ResumeWithoutPriorReviewJSON_LeavesPriorFindingsEmptyAndOmitsTriageInstruction(t *testing.T) {
	// Seed
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	prior := runstore.NewRunState("spec-no-prior", "proj-no-prior")
	prior.Status = runstore.StatusNeedsHuman
	if err := store.Save(prior); err != nil {
		t.Fatalf("save prior run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir(prior.RunID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("prepare evidence dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "review-outcome.json"), []byte(`{"outcome":"rework"}`), 0o644); err != nil {
		t.Fatalf("write review-outcome: %v", err)
	}

	agent := &promptCapturingReviewAgent{}
	var captured *runstore.RunState
	var capturedInput review.RunInput
	provider := &testStageProvider{stages: []specloop.Stage{
		&stageRecorderFunc{
			name: "review",
			fn: func(rs *runstore.RunState) {
				captured = rs
				if _, err := os.Stat(filepath.Join(evidenceDir, "review.json")); !os.IsNotExist(err) {
					t.Fatalf("precondition: review.json should not exist, got: %v", err)
				}
				runner := review.NewRunner(agent, review.RunnerConfig{
					Facets:    []string{"spec_alignment"},
					Threshold: review.SeverityError,
				})
				capturedInput = review.RunInput{
					SpecContent: "spec content",
					DiffSummary: "diff content",
					Cycle:       rs.Cycle,
				}
				_, err := runner.Run(context.Background(), capturedInput)
				if err != nil {
					t.Fatalf("run review runner: %v", err)
				}
			},
		},
	}}

	// Invoke
	run := &execSpecRun{
		specPath:      "spec.md",
		projectID:     "proj-no-prior",
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
	if captured == nil {
		t.Fatal("stage did not capture run state")
	}
	if !captured.Resumed {
		t.Fatal("expected resumed run state")
	}
	if len(capturedInput.PriorFindings) != 0 {
		t.Fatalf("expected no prior findings, got %d", len(capturedInput.PriorFindings))
	}
	if strings.Contains(agent.prompt, "Do NOT re-raise a prior finding unless") {
		t.Fatalf("expected prompt to omit prior-finding triage instruction, got:\n%s", agent.prompt)
	}
}
