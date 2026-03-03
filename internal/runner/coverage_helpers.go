package runner

import (
	"context"
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// populateCoverageResult populates coverage fields in bc.Result from the tracker.
func populateCoverageResult(bc *runtypes.BeadContext, tracker *coverage.CoverageTracker) {
	if bc == nil || bc.Result == nil || tracker == nil {
		return
	}

	bc.Result.CriteriaTotal = tracker.TotalCriteria()
	coveredCriteria := tracker.CoveredCriteria()
	bc.Result.CriteriaCovered = len(coveredCriteria)

	untestableCriteria := tracker.UntestableCriteria()
	bc.Result.CriteriaUntestable = len(untestableCriteria)

	uncoveredCriteria := tracker.UncoveredCriteria()
	bc.Result.UncoveredCriteria = make([]string, len(uncoveredCriteria))
	for i, criterion := range uncoveredCriteria {
		bc.Result.UncoveredCriteria[i] = criterion.Text
	}
}

// addCoverageCommentWithClient adds a bead comment with the coverage summary
// when there are uncovered or untestable criteria.
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

// logCoverageSummary logs the coverage summary to the provided writer when
// there are uncovered or untestable criteria.
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
