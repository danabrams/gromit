package policy

import (
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
)

const (
	methodologyATDD = "atdd"
	methodologyTDD  = "tdd"

	specLabelPrefix = "spec:"

	minRefactorBudget     = 60 * time.Second
	minRevalidationBudget = 30 * time.Second
)

// MethodologyPolicy decides methodology activation, phase timeouts, deadline
// guards, and post-success deferral.
type MethodologyPolicy interface {
	IsActive(labels []string, methodology string) bool
	PhaseTimeout(phase string, beadTimeoutSec int) int
	MinRefactorBudget() time.Duration
	MinRevalidationBudget() time.Duration
	ShouldDeferPostSuccess(atddActive, tddActive bool) bool
}

// ConfigMethodologyPolicy implements MethodologyPolicy backed by *config.Config.
type ConfigMethodologyPolicy struct {
	cfg *config.Config
}

var _ MethodologyPolicy = (*ConfigMethodologyPolicy)(nil)

// NewConfigMethodologyPolicy returns a MethodologyPolicy backed by cfg.
func NewConfigMethodologyPolicy(cfg *config.Config) MethodologyPolicy {
	return &ConfigMethodologyPolicy{cfg: cfg}
}

// ResolvedMethodologyAdapterValue exposes the resolved methodology adapter selector
// for policy consumers needing profile-aware knowledge.
func (p *ConfigMethodologyPolicy) ResolvedMethodologyAdapterValue() string {
	if p == nil || p.cfg == nil {
		return ""
	}
	return p.cfg.ResolveProfileDependentDefaults().MethodologyAdapter.Value
}

// IsActive checks whether the named methodology is active for the given bead
// labels, falling back to the global config default when no label overrides it.
func (p *ConfigMethodologyPolicy) IsActive(labels []string, methodology string) bool {
	if p == nil || p.ResolvedMethodologyAdapterValue() != "go" {
		return false
	}

	if methodology == methodologyATDD && p.cfg.Methodology.Granularity == config.MethodologyGranularitySpec {
		for _, label := range labels {
			if strings.HasPrefix(label, specLabelPrefix) {
				return false
			}
		}
	}

	trueLabel := methodology + ":true"
	falseLabel := methodology + ":false"
	for _, label := range labels {
		if label == trueLabel {
			return true
		}
		if label == falseLabel {
			return false
		}
	}
	switch methodology {
	case methodologyATDD:
		return p.cfg.Methodology.ATDD
	case methodologyTDD:
		return p.cfg.Methodology.TDD
	}
	return false
}

// PhaseTimeout delegates to the config methodology phase timeout resolver.
func (p *ConfigMethodologyPolicy) PhaseTimeout(phase string, beadTimeoutSec int) int {
	return p.cfg.Methodology.ResolvePhaseTimeoutSeconds(phase, beadTimeoutSec)
}

// MinRefactorBudget returns the minimum remaining bead budget required to start
// the refactor phase (matches the minRefactorTime constant).
func (p *ConfigMethodologyPolicy) MinRefactorBudget() time.Duration {
	return minRefactorBudget
}

// MinRevalidationBudget returns the minimum remaining bead budget required to
// run post-refactor re-validation (matches the minRevalidationTime constant).
func (p *ConfigMethodologyPolicy) MinRevalidationBudget() time.Duration {
	return minRevalidationBudget
}

// ShouldDeferPostSuccess returns true when neither atdd nor tdd is active,
// meaning post-success stages (review/learning) should run immediately.
func (p *ConfigMethodologyPolicy) ShouldDeferPostSuccess(atddActive, tddActive bool) bool {
	return !atddActive && !tddActive
}
