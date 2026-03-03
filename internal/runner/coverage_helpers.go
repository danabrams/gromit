package runner

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func populateCoverageResult(bc *runtypes.BeadContext, tracker *coverage.CoverageTracker) {
	if bc == nil || bc.Result == nil || tracker == nil {
		return
	}

	bc.Result.CriteriaTotal = tracker.TotalCriteria()
	bc.Result.CriteriaCovered = len(tracker.CoveredCriteria())
	bc.Result.CriteriaUntestable = len(tracker.UntestableCriteria())

	uncovered := tracker.UncoveredCriteria()
	bc.Result.UncoveredCriteria = make([]string, len(uncovered))
	for i, criterion := range uncovered {
		bc.Result.UncoveredCriteria[i] = criterion.Text
	}
}

func addCoverageCommentWithClient(ctx context.Context, bc *runtypes.BeadContext, tracker *coverage.CoverageTracker, beadsClient interface {
	AddComment(ctx context.Context, id, comment string) error
}) {
	if bc == nil || bc.Bead == nil || tracker == nil || beadsClient == nil {
		return
	}

	uncovered := tracker.UncoveredCriteria()
	untestable := tracker.UntestableCriteria()
	if len(uncovered) == 0 && len(untestable) == 0 {
		return
	}

	summary := tracker.Summary()
	_ = beadsClient.AddComment(ctx, bc.Bead.ID, summary)
}

func logCoverageSummary(output io.Writer, tracker *coverage.CoverageTracker) {
	if output == nil || tracker == nil {
		return
	}

	uncovered := tracker.UncoveredCriteria()
	untestable := tracker.UntestableCriteria()
	if len(uncovered) == 0 && len(untestable) == 0 {
		return
	}

	summary := tracker.Summary()
	fmt.Fprintf(output, "Coverage Summary: %s\n", summary)
}
