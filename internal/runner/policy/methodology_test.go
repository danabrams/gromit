package policy_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/policy"
)

func newMethodologyPolicy(atdd, tdd bool) policy.MethodologyPolicy {
	cfg := &config.Config{}
	cfg.Methodology.ATDD = atdd
	cfg.Methodology.TDD = tdd
	return policy.NewConfigMethodologyPolicy(cfg)
}

func TestIsActive_TrueLabelOverridesGlobalFalse(t *testing.T) {
	p := newMethodologyPolicy(false, false)
	labels := []string{"tdd:true"}
	if !p.IsActive(labels, "tdd") {
		t.Error("expected IsActive to return true when tdd:true label present")
	}
}

func TestIsActive_FalseLabelOverridesGlobalTrue(t *testing.T) {
	p := newMethodologyPolicy(false, true)
	labels := []string{"tdd:false"}
	if p.IsActive(labels, "tdd") {
		t.Error("expected IsActive to return false when tdd:false label present")
	}
}

func TestIsActive_FallsBackToGlobalConfig(t *testing.T) {
	pEnabled := newMethodologyPolicy(false, true)
	pDisabled := newMethodologyPolicy(false, false)
	labels := []string{"unrelated:label"}
	if !pEnabled.IsActive(labels, "tdd") {
		t.Error("expected IsActive to return true from global config when no matching label")
	}
	if pDisabled.IsActive(labels, "tdd") {
		t.Error("expected IsActive to return false from global config when no matching label")
	}
}

func TestPhaseTimeout_ConfiguredPhaseTimeoutUsed(t *testing.T) {
	cfg := &config.Config{}
	cfg.Methodology.PhaseTimeouts.RedSeconds = 120
	cfg.Methodology.PhaseTimeouts.GreenSeconds = 90
	cfg.Methodology.PhaseTimeouts.RefactorSeconds = 60
	p := policy.NewConfigMethodologyPolicy(cfg)

	if got := p.PhaseTimeout("red", 300); got != 120 {
		t.Errorf("PhaseTimeout(red): got %d, want 120", got)
	}
	if got := p.PhaseTimeout("green", 300); got != 90 {
		t.Errorf("PhaseTimeout(green): got %d, want 90", got)
	}
	if got := p.PhaseTimeout("refactor", 300); got != 60 {
		t.Errorf("PhaseTimeout(refactor): got %d, want 60", got)
	}
}

func TestPhaseTimeout_FallsBackToBeadTimeoutWhenUnconfigured(t *testing.T) {
	cfg := &config.Config{} // zero PhaseTimeouts
	p := policy.NewConfigMethodologyPolicy(cfg)

	got := p.PhaseTimeout("red", 500)
	if got != 500 {
		t.Errorf("PhaseTimeout with zero config: got %d, want 500 (bead timeout fallback)", got)
	}
}

func TestIsActive_ATDDGlobalConfig(t *testing.T) {
	p := newMethodologyPolicy(true, false)
	labels := []string{}
	if !p.IsActive(labels, "atdd") {
		t.Error("expected IsActive to return true for atdd from global config")
	}
	if p.IsActive(labels, "tdd") {
		t.Error("expected IsActive to return false for tdd from global config")
	}
}
