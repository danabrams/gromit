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
