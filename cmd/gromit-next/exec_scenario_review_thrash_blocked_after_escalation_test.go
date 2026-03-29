package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestExecScenarioReviewThrashBlockedAfterEscalation verifies that a thrash finding
// that blocks three times in a row terminates the run with blocked status and
// no additional task cycles.
func TestExecScenarioReviewThrashBlockedAfterEscalation(t *testing.T) {
	tmp := t.TempDir()

	specPath := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(specPath, []byte("# spec\n"), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	thrashFinding := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityError,
		File:        "thrash.go",
		Line:        42,
		Description: "thrash failure",
	}
	failureString := review.ReviewFailuresToStrings([]review.Finding{thrashFinding})[0]

	taskRunner := &thrashScenarioTaskRunner{}
	provider := &reviewThrashScenarioStageProvider{
		storeDir:      tmp,
		taskRunner:    taskRunner,
		failureString: failureString,
		reviewRunner:  &blockingThrashReviewRunner{thrashFinding: thrashFinding},
	}

	store := runstore.NewStore(tmp)
	run := &execSpecRun{
		specPath:      specPath,
		projectID:     "proj-thrash",
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}
	if err := run.run(context.Background()); err != nil {
		t.Fatalf("run spec: %v", err)
	}

	runs, err := store.List("proj-thrash")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	blockedRun := runs[0]
	if blockedRun.Status != runstore.StatusBlocked {
		t.Fatalf("expected blocked status, got %s", blockedRun.Status)
	}
	if blockedRun.BlockerSummary != failureString {
		t.Fatalf("unexpected blocker summary: %q", blockedRun.BlockerSummary)
	}
	if blockedRun.Cycle != 3 {
		t.Fatalf("expected cycle 3, got %d", blockedRun.Cycle)
	}
	if blockedRun.EndedAt.IsZero() {
		t.Fatalf("expected EndedAt to be set")
	}
	if !blockedRun.IsTerminal() {
		t.Fatalf("expected run to be terminal")
	}

	var thrashRuns []runstore.Task
	for _, seen := range taskRunner.history {
		if seen.TaskID == "task-thrash" {
			thrashRuns = append(thrashRuns, seen)
		}
	}
	if len(thrashRuns) != 3 {
		t.Fatalf("expected thrash task to run three times, got %d", len(thrashRuns))
	}

	fp := thrashFingerprintForTest(thrashFinding)
	counts := blockedRun.ReviewThrashCounts
	if counts == nil {
		t.Fatalf("expected thrash count map to be persisted")
	}
	if counts[fp] != 3 {
		t.Fatalf("expected thrash count 3, got %d", counts[fp])
	}
}

// blockingThrashReviewRunner returns a blocking thrash finding on the first
// three cycles to trigger the terminal blocked state.
type blockingThrashReviewRunner struct {
	thrashFinding review.Finding
}

func (r *blockingThrashReviewRunner) Run(_ context.Context, input review.RunInput) (*review.RunResult, error) {
	result := &review.RunResult{
		FindingsByFacet: map[string][]review.Finding{},
		ErroredFacets:   map[string]string{},
	}
	if input.Cycle <= 3 {
		rec := r.thrashFinding
		rec.Cycle = input.Cycle
		result.AllFindings = []review.Finding{rec}
		result.BlockingFindings = []review.Finding{rec}
		result.FindingsByFacet[rec.Facet] = []review.Finding{rec}
		result.HasBlockingFindings = true
	}
	result.NormalizeNilFields()
	return result, nil
}

func thrashFingerprintForTest(f review.Finding) string {
	return f.File + "\x00" + f.Description
}
