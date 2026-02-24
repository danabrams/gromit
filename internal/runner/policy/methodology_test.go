package policy_test

import (
	"testing"
	"time"

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

func TestMinRefactorBudget_Returns60Seconds(t *testing.T) {
	p := newMethodologyPolicy(false, false)
	want := 60 * time.Second
	if got := p.MinRefactorBudget(); got != want {
		t.Errorf("MinRefactorBudget: got %v, want %v", got, want)
	}
}

func TestMinRevalidationBudget_Returns30Seconds(t *testing.T) {
	p := newMethodologyPolicy(false, false)
	want := 30 * time.Second
	if got := p.MinRevalidationBudget(); got != want {
		t.Errorf("MinRevalidationBudget: got %v, want %v", got, want)
	}
}

func TestShouldDeferPostSuccess_WhenMethodologyActive(t *testing.T) {
	p := newMethodologyPolicy(false, false)

	// Both inactive: post-success should run immediately (no deferral = true)
	if !p.ShouldDeferPostSuccess(false, false) {
		t.Error("expected ShouldDeferPostSuccess=true when neither atdd nor tdd active")
	}
	// ATDD active: defer post-success
	if p.ShouldDeferPostSuccess(true, false) {
		t.Error("expected ShouldDeferPostSuccess=false when atdd active")
	}
	// TDD active: defer post-success
	if p.ShouldDeferPostSuccess(false, true) {
		t.Error("expected ShouldDeferPostSuccess=false when tdd active")
	}
	// Both active: defer post-success
	if p.ShouldDeferPostSuccess(true, true) {
		t.Error("expected ShouldDeferPostSuccess=false when both active")
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

func newMethodologyPolicyWithGranularity(atdd, tdd bool, granularity string) policy.MethodologyPolicy {
	cfg := &config.Config{}
	cfg.Methodology.ATDD = atdd
	cfg.Methodology.TDD = tdd
	cfg.Methodology.Granularity = granularity
	return policy.NewConfigMethodologyPolicy(cfg)
}

func TestIsActive_ATDDSuppressedWhenSpecGranularityAndSpecLabel(t *testing.T) {
	p := newMethodologyPolicyWithGranularity(true, false, config.MethodologyGranularitySpec)
	labels := []string{"spec:my-feature"}
	if p.IsActive(labels, "atdd") {
		t.Error("expected IsActive to return false for atdd when granularity=spec and spec label present")
	}
}

func TestIsActive_ATDDNotSuppressedWhenSpecGranularityButNoSpecLabel(t *testing.T) {
	p := newMethodologyPolicyWithGranularity(true, false, config.MethodologyGranularitySpec)
	labels := []string{"unrelated:label"}
	if !p.IsActive(labels, "atdd") {
		t.Error("expected IsActive to return true for atdd when granularity=spec but no spec label")
	}
}

func TestIsActive_ATDDNotSuppressedWhenBeadGranularityEvenWithSpecLabel(t *testing.T) {
	p := newMethodologyPolicyWithGranularity(true, false, config.MethodologyGranularityBead)
	labels := []string{"spec:my-feature"}
	if !p.IsActive(labels, "atdd") {
		t.Error("expected IsActive to return true for atdd when granularity=bead even with spec label")
	}
}

func TestIsActive_TDDNotSuppressedBySpecGranularity(t *testing.T) {
	p := newMethodologyPolicyWithGranularity(false, true, config.MethodologyGranularitySpec)
	labels := []string{"spec:my-feature"}
	if !p.IsActive(labels, "tdd") {
		t.Error("expected IsActive to return true for tdd even when granularity=spec with spec label")
	}
}

func TestIsActive_ReturnsFalseForNonGoMethodologyAdapter(t *testing.T) {
	cfg := &config.Config{}
	cfg.Methodology.TDD = true
	cfg.Methodology.Adapter = "python"
	p := policy.NewConfigMethodologyPolicy(cfg)

	if p.IsActive(nil, "tdd") {
		t.Fatal("expected IsActive to return false when methodology adapter is non-go")
	}
}
