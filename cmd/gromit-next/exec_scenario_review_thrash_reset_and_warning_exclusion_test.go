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

// TestExecScenarioReviewThrashResetAndWarningExclusion exercises the thrahs logic
// spanning multiple cycles: an error escalates, the finding disappears in the
// next cycle (replaced by a warning), and the error reappears again without
// escalating because the streak reset. It also verifies warning-severity
// findings do not contribute to thrash counts.
func TestExecScenarioReviewThrashResetAndWarningExclusion(t *testing.T) {
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
		Description: "thrash reset candidate",
	}
	warningFinding := review.Finding{
		Facet:       "spec_alignment",
		Severity:    review.SeverityWarning,
		File:        "thrash.go",
		Line:        100,
		Description: "non-blocking warning",
	}
	failureString := review.ReviewFailuresToStrings([]review.Finding{thrashFinding})[0]

	var countsLog []thrashCountsSnapshot
	taskRunner := &thrashScenarioTaskRunner{}
	provider := &reviewThrashScenarioStageProvider{
		storeDir:        tmp,
		taskRunner:      taskRunner,
		failureString:   failureString,
		reviewRunner:    &resetWarningReviewRunner{thrashFinding: thrashFinding, warningFinding: warningFinding},
		thrashCountsLog: &countsLog,
	}

	store := runstore.NewStore(tmp)
	run := &execSpecRun{
		specPath:      specPath,
		projectID:     "proj-thrash-reset",
		storeDir:      tmp,
		stageProvider: provider,
		policy:        ptrPolicy(execpolicy.DefaultPolicy()),
		store:         store,
		out:           io.Discard,
	}
	if err := run.run(context.Background()); err != nil {
		t.Fatalf("run spec: %v", err)
	}

	runs, err := store.List("proj-thrash-reset")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	eventsPath := filepath.Join(tmp, "runs", runs[0].RunID, "events.jsonl")
	events, err := runstore.NewEventLog(eventsPath).ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var thrashEvents []*runstore.ReviewThrashEscalatedEvent
	for _, ev := range events {
		if e, ok := ev.(*runstore.ReviewThrashEscalatedEvent); ok {
			thrashEvents = append(thrashEvents, e)
		}
	}
	if len(thrashEvents) != 1 {
		t.Fatalf("expected exactly 1 review_thrash_escalated event, got %d", len(thrashEvents))
	}
	if thrashEvents[0].ConsecutiveCount != 2 {
		t.Fatalf("expected thrash event count 2, got %d", thrashEvents[0].ConsecutiveCount)
	}

	const requiredCycles = 5
	if len(countsLog) != requiredCycles {
		t.Fatalf("expected %d recorded cycles, got %d", requiredCycles, len(countsLog))
	}

	fp := thrashFingerprintForTest(thrashFinding)
	checkCount := func(cycle, want int) {
		counts := snapshotForCycle(t, countsLog, cycle)
		if counts[fp] != want {
			t.Fatalf("cycle %d: expected thrash count %d, got %d", cycle, want, counts[fp])
		}
	}
	checkEmpty := func(cycle int) {
		counts := snapshotForCycle(t, countsLog, cycle)
		if len(counts) != 0 {
			t.Fatalf("cycle %d: expected empty thrash counts, got %v", cycle, counts)
		}
	}

	checkCount(2, 1) // after first error
	checkCount(3, 2) // after second (escalated) error
	checkEmpty(4)    // warning cycle resets the streak
	checkCount(5, 1) // error reappears but count resets to 1
}

func snapshotForCycle(t *testing.T, log []thrashCountsSnapshot, cycle int) map[string]int {
	t.Helper()
	for _, snapshot := range log {
		if snapshot.Cycle == cycle {
			if snapshot.Counts == nil {
				return map[string]int{}
			}
			return snapshot.Counts
		}
	}
	t.Fatalf("missing counts log entry for cycle %d", cycle)
	return nil
}

type resetWarningReviewRunner struct {
	thrashFinding  review.Finding
	warningFinding review.Finding
}

func (r *resetWarningReviewRunner) Run(_ context.Context, input review.RunInput) (*review.RunResult, error) {
	result := &review.RunResult{
		FindingsByFacet: map[string][]review.Finding{},
		ErroredFacets:   map[string]string{},
	}

	switch input.Cycle {
	case 1, 2, 4:
		rec := r.thrashFinding
		rec.Cycle = input.Cycle
		result.AllFindings = []review.Finding{rec}
		result.BlockingFindings = []review.Finding{rec}
		result.FindingsByFacet[rec.Facet] = []review.Finding{rec}
		result.HasBlockingFindings = true
	case 3:
		warn := r.warningFinding
		warn.Cycle = input.Cycle
		result.AllFindings = []review.Finding{warn}
		result.BlockingFindings = []review.Finding{warn}
		result.FindingsByFacet[warn.Facet] = []review.Finding{warn}
		result.HasBlockingFindings = true
	default:
		result.HasBlockingFindings = false
	}

	result.NormalizeNilFields()
	return result, nil
}
