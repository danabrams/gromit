package specmerge_test

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/internal/specgate"
)

type fakeBeadCreator struct {
	createFn func(ctx context.Context, title, description, priority string, labels []string) (string, error)
}

var _ specgate.BeadCreator = (*fakeBeadCreator)(nil)

func (f *fakeBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	if f.createFn == nil {
		return "", nil
	}
	return f.createFn(ctx, title, description, priority, labels)
}

func TestHandleStageFailure_CreateFixBeads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	failures := []specgate.CriterionResult{
		{Criterion: "Test quality", Passed: false, Evidence: "missing tests"},
	}

	var createdBeadIDs []string
	creator := &fakeBeadCreator{
		createFn: func(_ context.Context, title, description, priority string, labels []string) (string, error) {
			createdBeadIDs = append(createdBeadIDs, "bead-1")
			return "bead-1", nil
		},
	}

	deps := specmerge.FixBeadDependencies{
		BeadCreator: creator,
	}

	opts := specmerge.HandleStageFailureOptions{
		SpecName:      "test-spec",
		Failures:      failures,
		Priority:      "P1",
		AttemptCount:  0,
		RetryCap:      3,
	}

	err := specmerge.HandleStageFailure(ctx, deps, opts)
	if err != nil {
		t.Fatalf("HandleStageFailure returned error: %v", err)
	}

	if len(createdBeadIDs) != 1 {
		t.Fatalf("HandleStageFailure created %d beads, want 1", len(createdBeadIDs))
	}
}
