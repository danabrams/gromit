package policy_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func newConfigValidationPolicy(fullEveryN int) policy.ValidationPolicy {
	cfg := &config.Config{}
	cfg.Validation.FullValidationEveryN = fullEveryN
	return policy.NewConfigValidationPolicy(cfg)
}

func TestSelectGate_ReturnsFastOnNonModuloIteration(t *testing.T) {
	p := newConfigValidationPolicy(3)
	if got := p.SelectGate(1); got != policy.GateFast {
		t.Errorf("SelectGate(1) with fullEveryN=3: got %v, want GateFast", got)
	}
}

func TestSelectGate_ReturnsFullOnModuloBoundary(t *testing.T) {
	p := newConfigValidationPolicy(3)
	if got := p.SelectGate(3); got != policy.GateFull {
		t.Errorf("SelectGate(3) with fullEveryN=3: got %v, want GateFull", got)
	}
}

func TestSelectGate_ZeroConsecutiveSuccessesReturnsFullWhenEnabled(t *testing.T) {
	p := newConfigValidationPolicy(3)
	if got := p.SelectGate(0); got != policy.GateFull {
		t.Errorf("SelectGate(0) with fullEveryN=3: got %v, want GateFull (0%%3==0)", got)
	}
}

func TestSelectGate_ZeroFullEveryNAlwaysReturnsFast(t *testing.T) {
	p := newConfigValidationPolicy(0)
	for _, n := range []int{0, 1, 5, 100} {
		if got := p.SelectGate(n); got != policy.GateFast {
			t.Errorf("SelectGate(%d) with fullEveryN=0: got %v, want GateFast", n, got)
		}
	}
}

func TestMaxRecoveryAttempts_ReturnsTwo(t *testing.T) {
	p := newConfigValidationPolicy(0)
	if got := p.MaxRecoveryAttempts(); got != 2 {
		t.Errorf("MaxRecoveryAttempts: got %d, want 2", got)
	}
}
