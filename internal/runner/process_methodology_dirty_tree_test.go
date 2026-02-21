package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestRunTDDFreshContextCycles_ResetsOnRunCyclesError verifies that when
// RunCycles returns an error, resetHard is called with bc.StartCommit.
func TestRunTDDFreshContextCycles_ResetsOnRunCyclesError(t *testing.T) {
	r, _, _ := newTDDFreshContextCoverageHarness(
		t,
		func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			return errors.New("refactor phase invocation failed")
		},
		nil,
	)

	var resetCommit string
	r.resetHardFn = func(commit string) error {
		resetCommit = commit
		return nil
	}

	_, bc := newCoverageBeadContext("tdd-dirty-tree-1", "Implement feature with refactor failure", authSpecOneCriterion)
	bc.StartCommit = "abc123"

	handled := r.runTDDFreshContextCycles(context.Background(), bc)

	if !handled {
		t.Fatal("expected runTDDFreshContextCycles to return handled=true on error")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected bc.Result.Error to be set on RunCycles failure")
	}
	if resetCommit != "abc123" {
		t.Errorf("resetHard called with %q, want %q", resetCommit, "abc123")
	}
}

// TestRunTDDFreshContextCycles_ResetsOnCoverageIncomplete verifies that when
// the coverage tracker reports incomplete coverage after max passes,
// resetHard is called with bc.StartCommit.
func TestRunTDDFreshContextCycles_ResetsOnCoverageIncomplete(t *testing.T) {
	// RunCycles succeeds but never marks criteria covered, leaving tracker incomplete.
	r, _, _ := newTDDFreshContextCoverageHarness(t, nil, nil)

	var resetCommit string
	r.resetHardFn = func(commit string) error {
		resetCommit = commit
		return nil
	}

	_, bc := newCoverageBeadContext("tdd-dirty-tree-2", "Implement feature with incomplete coverage", authSpecOneCriterion)
	bc.StartCommit = "def456"

	handled := r.runTDDFreshContextCycles(context.Background(), bc)

	if !handled {
		t.Fatal("expected runTDDFreshContextCycles to return handled=true on coverage incomplete")
	}
	if bc.Result.Error == nil {
		t.Fatal("expected bc.Result.Error to be set when coverage is incomplete")
	}
	if resetCommit != "def456" {
		t.Errorf("resetHard called with %q, want %q", resetCommit, "def456")
	}
}

// TestRunTDDFreshContextCycles_NoResetWhenStartCommitEmpty verifies that
// resetHard is not called when bc.StartCommit is empty.
func TestRunTDDFreshContextCycles_NoResetWhenStartCommitEmpty(t *testing.T) {
	r, _, _ := newTDDFreshContextCoverageHarness(
		t,
		func(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, criteria []coverage.Criterion) error {
			return errors.New("failure")
		},
		nil,
	)

	resetCalled := false
	r.resetHardFn = func(commit string) error {
		resetCalled = true
		return nil
	}

	_, bc := newCoverageBeadContext("tdd-dirty-tree-3", "Implement feature no start commit", authSpecOneCriterion)
	bc.StartCommit = "" // no start commit

	r.runTDDFreshContextCycles(context.Background(), bc)

	if resetCalled {
		t.Error("resetHard should not be called when StartCommit is empty")
	}
}
