package runner

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/logger"
)

// TestExecuteBuildLoop_SetsFailurePhaseBuild verifies that executeBuildAndMethodologyLoop
// sets FailurePhase to the build constant when executeWithRetry returns false.
func TestExecuteBuildLoop_SetsFailurePhaseBuild(t *testing.T) {
	r, _ := newMinimalRunnerForMethodology(t, nil, &mockPromptRenderer{})
	b := newTestBead("fp-build-1", "Feature bead")
	bc := newBeadContextForMethodology(b)

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool { return false }, // build always fails
	)

	if result.FailurePhase != logger.FailurePhaseBuild {
		t.Errorf("FailurePhase = %q, want %q", result.FailurePhase, logger.FailurePhaseBuild)
	}
}
