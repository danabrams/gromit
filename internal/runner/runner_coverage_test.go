package runner

import (
	"bytes"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestRunnerReportCoverageAddsCommentAndLogsWhenCoverageIncomplete(t *testing.T) {
	tracker := coverage.NewTracker([]coverage.Criterion{
		{Number: 1, Text: "Criterion 1"},
		{Number: 2, Text: "Criterion 2"},
	}, 3)
	tracker.MarkCovered(1)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)
	tracker.RecordRejection(2)

	var output bytes.Buffer
	var loggedComment string

	beadsClient := &captureBeadsClient{
		addCommentFn: func(id, comment string) error {
			loggedComment = comment
			return nil
		},
	}

	runner := &Runner{
		beads:  beadsClient,
		output: &output,
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "bead-coverage"},
		Result: &runtypes.IterationResult{},
	}

	runner.reportCoverage(bc, tracker)

	if bc.Result.CriteriaTotal != 2 {
		t.Fatalf("CriteriaTotal = %d, want 2", bc.Result.CriteriaTotal)
	}
	if bc.Result.CriteriaCovered != 1 {
		t.Fatalf("CriteriaCovered = %d, want 1", bc.Result.CriteriaCovered)
	}
	if bc.Result.CriteriaUntestable != 1 {
		t.Fatalf("CriteriaUntestable = %d, want 1", bc.Result.CriteriaUntestable)
	}
	if len(bc.Result.UncoveredCriteria) != 1 {
		t.Fatalf("len(UncoveredCriteria) = %d, want 1", len(bc.Result.UncoveredCriteria))
	}
	if loggedComment == "" {
		t.Fatalf("expected bead comment, got none")
	}
	if !bytes.Contains(output.Bytes(), []byte("Coverage Summary:")) {
		t.Fatalf("expected coverage summary log, got %q", output.String())
	}
}

type captureBeadsClient struct {
	addCommentFn func(id, comment string) error
}

func (c *captureBeadsClient) AddComment(id, comment string) error {
	if c.addCommentFn == nil {
		return nil
	}
	return c.addCommentFn(id, comment)
}
