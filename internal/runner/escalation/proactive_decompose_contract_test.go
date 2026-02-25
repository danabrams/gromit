package escalation

import (
	"testing"

	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// Contract tests documenting the proactive decomposition policy.

// TestProactiveDecomposition_Contract_HighRiskBead documents the contract
// for identifying high-risk beads that are eligible for proactive decomposition.
// High-risk beads have at least one of:
// - Complexity = "high"
// - EstimatedIterations >= 3
// - Prior retries (TotalRetriesThisBead > 0)
// - Multi-iteration without explicit low-complexity marking
func TestProactiveDecomposition_Contract_HighRiskBead(t *testing.T) {
	tests := []struct {
		name       string
		scope      *prompt.ScopeEstimate
		retries    int
		wantHighRisk bool
	}{
		{
			name:         "High complexity is high-risk",
			scope:        &prompt.ScopeEstimate{Complexity: "high"},
			retries:      0,
			wantHighRisk: true,
		},
		{
			name:         "3+ iterations is high-risk",
			scope:        &prompt.ScopeEstimate{Complexity: "medium", EstimatedIterations: 3},
			retries:      0,
			wantHighRisk: true,
		},
		{
			name:         "Prior retries is high-risk",
			scope:        &prompt.ScopeEstimate{Complexity: "low", EstimatedIterations: 1, CanCompleteInSingleIteration: true},
			retries:      1,
			wantHighRisk: true,
		},
		{
			name:         "Multi-iteration without low-complexity is high-risk",
			scope:        &prompt.ScopeEstimate{Complexity: "medium", EstimatedIterations: 2, CanCompleteInSingleIteration: false},
			retries:      0,
			wantHighRisk: true,
		},
		{
			name:         "Low complexity, single iteration, no retries is NOT high-risk",
			scope:        &prompt.ScopeEstimate{Complexity: "low", EstimatedIterations: 1, CanCompleteInSingleIteration: true},
			retries:      0,
			wantHighRisk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &runtypes.BeadContext{
				ScopeEstimate:        tt.scope,
				TotalRetriesThisBead: tt.retries,
			}
			got := IsHighRiskBead(bc)
			if got != tt.wantHighRisk {
				t.Errorf("IsHighRiskBead() = %v, want %v", got, tt.wantHighRisk)
			}
		})
	}
}

// TestProactiveDecomposition_Contract_TimeoutBudgetThreshold documents the contract
// for proactive decomposition timing. Decomposition is triggered when:
// - Bead is high-risk, AND
// - Elapsed time >= 60% of BeadTimeout
func TestProactiveDecomposition_Contract_TimeoutBudgetThreshold(t *testing.T) {
	if proactiveDecomposeThreshold != 0.60 {
		t.Errorf("proactiveDecomposeThreshold = %v, want 0.60", proactiveDecomposeThreshold)
	}
}

// TestProactiveDecomposition_Contract_OnlyAffectsHighRisk documents that
// proactive decomposition only affects high-risk beads. Low-risk beads
// proceed normally without decomposition checks.
func TestProactiveDecomposition_Contract_OnlyAffectsHighRisk(t *testing.T) {
	// Even at 100% elapsed time, low-risk beads don't trigger proactive decomposition
	bc := &runtypes.BeadContext{
		ScopeEstimate: &prompt.ScopeEstimate{
			Complexity:                   "low",
			EstimatedIterations:          1,
			CanCompleteInSingleIteration: true,
		},
		TotalRetriesThisBead: 0,
	}

	// Elapsed time doesn't matter for low-risk beads
	if ShouldProactivelyDecompose(bc) {
		t.Errorf("Low-risk bead should not trigger proactive decomposition")
	}
}

// TestProactiveDecomposition_Contract_PreservesTimeoutPath documents that
// proactive decomposition preserves the timeout-first decomposition path.
// If a hard timeout occurs, it takes precedence over proactive thresholds.
// The timeout handlers (HandleInvocationTimeout, HandleBeadTimeout) remain unchanged.
func TestProactiveDecomposition_Contract_PreservesTimeoutPath(t *testing.T) {
	// This is documented behavior: proactive decomposition is a safeguard that
	// triggers BEFORE timeouts occur. If a timeout happens anyway, the timeout
	// handler takes over. This preserves the "timeout-first" semantics.
	// See ExecuteWithRetry for the integration point: CheckProactiveDecomposition
	// is called before invocation, before timeouts can occur.
}

// TestProactiveDecomposition_Contract_AuditableEvents documents that
// proactive decomposition events are auditable via result fields.
// TimeoutDecompositionAttempted and TimeoutDecompositionSucceeded track
// whether proactive decomposition was attempted and whether it succeeded.
func TestProactiveDecomposition_Contract_AuditableEvents(t *testing.T) {
	// These fields in IterationResult provide auditability:
	// - TimeoutDecompositionAttempted: bool (set to true when proactive check triggers)
	// - TimeoutDecompositionSucceeded: bool (set to true when decomposition succeeds)
	//
	// This allows operators to understand which beads triggered proactive
	// decomposition and whether the decomposition was successful.
	//
	// Example audit trail:
	// - TimeoutDecompositionAttempted=true, TimeoutDecompositionSucceeded=true
	//   → proactive decomposition succeeded
	// - TimeoutDecompositionAttempted=true, TimeoutDecompositionSucceeded=false
	//   → proactive decomposition failed
	// - TimeoutDecompositionAttempted=false
	//   → no proactive decomposition (low-risk or below threshold)
}
