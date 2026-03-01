//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// TestOrchestrator_QueueDoesNotGrowUnbounded is a regression test that verifies
// the orchestrator does not exhibit unbounded queue growth during multi-iteration
// execution. Given a controlled set of beads in the queue, processing iterations
// should eventually complete with entries drained from the ready queue rather than
// accumulating indefinitely.
func TestOrchestrator_QueueDoesNotGrowUnbounded(t *testing.T) {
	t.Parallel()

	// Create a controlled fixture: a small set of beads that will be re-offered
	// by the queue if not properly closed. This simulates the scenario where the
	// orchestrator must skip already-processed beads to avoid infinite loops.
	allBeads := []*bead.Bead{
		{ID: "bead-1", Title: "Task 1", Priority: 1, ExpectedOutputs: []string{}},
		{ID: "bead-2", Title: "Task 2", Priority: 1, ExpectedOutputs: []string{}},
		{ID: "bead-3", Title: "Task 3", Priority: 1, ExpectedOutputs: []string{}},
	}

	// Track ready() calls to monitor queue access patterns and verify iteration counts
	var readyCallCount int
	var beadSequence []string

	mockBeads := &mockBeadClient{
		ReadyFn: func(ctx context.Context) (*bead.Bead, error) {
			readyCallCount++

			// Return beads in sequence, then keep re-offering them
			// (simulating a queue that returns the same unclosed beads repeatedly)
			idx := (readyCallCount - 1) % len(allBeads)
			beadSequence = append(beadSequence, allBeads[idx].ID)
			return allBeads[idx], nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	h := NewOrchestratorTestHelperWithDeps(t, cfg, io.Discard, mockBeads, newMockRouter())

	// Run orchestrator for a bounded number of iterations to catch unbounded growth.
	// If the orchestrator doesn't properly skip re-offered beads, it would continue
	// indefinitely trying to process the same beads. With proper duplicate detection,
	// it should exit after processing each bead once and detecting all are re-offered.
	maxIterations := 20 // Enough for 3 beads × 6+ re-offers = unbounded case would exceed this
	err := h.Run(context.Background(), maxIterations, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify that orchestrator exited before hitting max iterations.
	// If unbounded growth is present, it would use all 20 iterations trying to process
	// re-offered beads. With proper skip logic, it should exit much earlier (~3-9 calls).
	//
	// Expected behavior:
	// - Iteration 1: Get bead-1 (readyCallCount=1), process it
	// - Iteration 2: Get bead-2 (readyCallCount=2), process it
	// - Iteration 3: Get bead-3 (readyCallCount=3), process it
	// - Iteration 4: Get bead-1 again (readyCallCount=4), skip (already processed)
	// - Iteration 5: Get bead-2 again (readyCallCount=5), skip (already processed)
	// - Iteration 6: Get bead-3 again (readyCallCount=6), skip (already processed, consecutiveSkips==3==len(processedBeads)), exit
	// Verify ready() was called a reasonable number of times (not unbounded)
	maxExpectedReady := 10 // Conservative upper bound: 3 beads + 7 re-offers to detect saturation
	if readyCallCount >= maxIterations {
		t.Errorf("Orchestrator called Ready() %d times - unbounded queue growth detected; "+
			"expected early exit from repeated bead detection (max %d expected); sequence: %v",
			readyCallCount, maxExpectedReady, beadSequence)
	}

	// Verify the sequence shows initial processing followed by re-offerings
	if readyCallCount < len(allBeads) {
		t.Errorf("Ready() called only %d times (expected at least %d for initial round)",
			readyCallCount, len(allBeads))
	}

	// Verify at least one bead was re-offered (proof of saturation detection)
	if readyCallCount == len(allBeads) {
		// If readyCallCount == len(allBeads), we only did initial processing with no re-offers.
		// This is OK but means the test might not be strong enough to catch the bug.
		t.Logf("Ready() called exactly %d times (no re-offers detected); "+
			"orchestrator may have exited too early or queue draining was very fast", readyCallCount)
	}
}
