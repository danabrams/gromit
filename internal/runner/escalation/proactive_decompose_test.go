package escalation

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestRiskScore_HighComplexity verifies that high-complexity scope
// is recognized as a risk factor for proactive decomposition.
func TestRiskScore_HighComplexity(t *testing.T) {
	bc := &runtypes.BeadContext{
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "high",
			EstimatedIterations:          2,
			CanCompleteInSingleIteration: false,
		},
		TotalRetriesThisBead: 0,
		BeadTimeout:          5 * time.Minute,
		BeadStartTime:        time.Now().Add(-1 * time.Minute),
	}

	isHighRisk := IsHighRiskBead(bc)
	if !isHighRisk {
		t.Errorf("IsHighRiskBead() = false, want true for high-complexity bead")
	}
}

// TestRiskScore_ManyIterations verifies that estimated_iterations >= 3
// is recognized as a risk factor.
func TestRiskScore_ManyIterations(t *testing.T) {
	bc := &runtypes.BeadContext{
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "medium",
			EstimatedIterations:          3,
			CanCompleteInSingleIteration: false,
		},
		TotalRetriesThisBead: 0,
		BeadTimeout:          5 * time.Minute,
		BeadStartTime:        time.Now().Add(-1 * time.Minute),
	}

	isHighRisk := IsHighRiskBead(bc)
	if !isHighRisk {
		t.Errorf("IsHighRiskBead() = false, want true for 3+ estimated iterations")
	}
}

// TestRiskScore_PreviousRetries verifies that prior retries are recognized
// as a risk factor.
func TestRiskScore_PreviousRetries(t *testing.T) {
	bc := &runtypes.BeadContext{
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "low",
			EstimatedIterations:          1,
			CanCompleteInSingleIteration: true,
		},
		TotalRetriesThisBead: 1,
		BeadTimeout:          5 * time.Minute,
		BeadStartTime:        time.Now().Add(-1 * time.Minute),
	}

	isHighRisk := IsHighRiskBead(bc)
	if !isHighRisk {
		t.Errorf("IsHighRiskBead() = false, want true for bead with prior retries")
	}
}

// TestRiskScore_NotHighRisk verifies that a low-risk bead is not flagged
// as high-risk.
func TestRiskScore_NotHighRisk(t *testing.T) {
	bc := &runtypes.BeadContext{
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "low",
			EstimatedIterations:          1,
			CanCompleteInSingleIteration: true,
		},
		TotalRetriesThisBead: 0,
		BeadTimeout:          5 * time.Minute,
		BeadStartTime:        time.Now().Add(-1 * time.Minute),
	}

	isHighRisk := IsHighRiskBead(bc)
	if isHighRisk {
		t.Errorf("IsHighRiskBead() = true, want false for low-risk bead")
	}
}

// TestProactiveDecomposeThreshold_60Percent verifies that a high-risk bead
// reaches the 60% elapsed budget threshold and triggers proactive decomposition.
func TestProactiveDecomposeThreshold_60Percent(t *testing.T) {
	bc := &runtypes.BeadContext{
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "high",
			EstimatedIterations:          2,
			CanCompleteInSingleIteration: false,
		},
		TotalRetriesThisBead: 0,
		BeadTimeout:          10 * time.Minute,
		BeadStartTime:        time.Now().Add(-6 * time.Minute), // 60% elapsed
	}

	shouldDecompose := ShouldProactivelyDecompose(bc)
	if !shouldDecompose {
		t.Errorf("ShouldProactivelyDecompose() = false, want true at 60%% elapsed for high-risk bead")
	}
}

// TestProactiveDecomposeThreshold_Below60Percent verifies that a high-risk bead
// below 60% elapsed does not trigger proactive decomposition.
func TestProactiveDecomposeThreshold_Below60Percent(t *testing.T) {
	bc := &runtypes.BeadContext{
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "high",
			EstimatedIterations:          2,
			CanCompleteInSingleIteration: false,
		},
		TotalRetriesThisBead: 0,
		BeadTimeout:          10 * time.Minute,
		BeadStartTime:        time.Now().Add(-5 * time.Minute), // 50% elapsed
	}

	shouldDecompose := ShouldProactivelyDecompose(bc)
	if shouldDecompose {
		t.Errorf("ShouldProactivelyDecompose() = true, want false below 60%% elapsed")
	}
}

// TestProactiveDecomposeThreshold_LowRiskNotTriggered verifies that low-risk beads
// do not trigger proactive decomposition even at 60%+ elapsed.
func TestProactiveDecomposeThreshold_LowRiskNotTriggered(t *testing.T) {
	bc := &runtypes.BeadContext{
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "low",
			EstimatedIterations:          1,
			CanCompleteInSingleIteration: true,
		},
		TotalRetriesThisBead: 0,
		BeadTimeout:          10 * time.Minute,
		BeadStartTime:        time.Now().Add(-6 * time.Minute), // 60% elapsed
	}

	shouldDecompose := ShouldProactivelyDecompose(bc)
	if shouldDecompose {
		t.Errorf("ShouldProactivelyDecompose() = true, want false for low-risk bead")
	}
}

// TestCheckProactiveDecomposition_TriggersDecomposition verifies that the Handler
// detects when proactive decomposition should occur and initiates decomposition.
func TestCheckProactiveDecomposition_TriggersDecomposition(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	decomposeCalled := false
	decomposeFn := func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
		decomposeCalled = true
		return []runtypes.SubTask{}, nil
	}

	createSubCalled := false
	createSubFn := func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
		createSubCalled = true
		return nil
	}

	handler := NewHandler(cfg, nil, nil, decomposeFn, createSubFn, nil, nil)

	bc := &runtypes.BeadContext{
		Bead: &bead.Bead{
			ID:       "test-001",
			Title:    "High-risk test bead",
			Priority: 1,
		},
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "high",
			EstimatedIterations:          2,
			CanCompleteInSingleIteration: false,
		},
		TotalRetriesThisBead: 0,
		BeadTimeout:          10 * time.Minute,
		BeadStartTime:        time.Now().Add(-6 * time.Minute), // 60% elapsed
		Result:               &runtypes.IterationResult{},
		ParentCtx:            context.Background(),
	}

	ctx := context.Background()
	continueLoop := handler.CheckProactiveDecomposition(ctx, bc)

	if continueLoop {
		t.Errorf("CheckProactiveDecomposition() = true, want false (should attempt decomposition)")
	}
	if !decomposeCalled {
		t.Errorf("decomposeFn not called, expected it to be called for proactive decomposition")
	}
	if !createSubCalled {
		t.Errorf("createSubFn not called, expected it to be called after decomposition")
	}
}
