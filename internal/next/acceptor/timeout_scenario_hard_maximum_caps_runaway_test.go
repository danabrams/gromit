package acceptor

import (
	"testing"
	"time"
)

func TestScenario_ComputeCriterionTimeout_HardMaximumCapsRunaway(t *testing.T) {
	// Seed
	cfg := DefaultTimeoutConfig()
	diffBytes := 3_000_000
	criterion := "integration test passes across all scenarios"

	base := time.Duration(cfg.BaseSeconds) * time.Second
	diffComponent := time.Duration(diffBytes/cfg.RateConstant) * time.Second
	complexityBonus := time.Duration(cfg.ComplexityBonusSecs) * time.Second
	uncapped := base + diffComponent + complexityBonus
	hardMax := time.Duration(cfg.HardMaximumSecs) * time.Second

	// Invoke
	got := ComputeCriterionTimeout(cfg, diffBytes, criterion)

	// Assert
	if uncapped != 780*time.Second {
		t.Fatalf("uncapped timeout %v, want 13m0s (780s)", uncapped)
	}
	if got != hardMax {
		t.Fatalf("computed timeout %v, want hard maximum %v", got, hardMax)
	}
	if got != 600*time.Second {
		t.Fatalf("computed timeout %v, want 10m0s (600s)", got)
	}
}
