package policy

import "github.com/danabrams/gromit/internal/config"

// GateType selects which validation command set to run.
type GateType int

const (
	GateFast GateType = iota
	GateFull
)

// ValidationPolicy decides gate type, recovery attempt bounds, and mandatory
// command coverage.
type ValidationPolicy interface {
	SelectGate(consecutiveSuccesses int) GateType
	MaxRecoveryAttempts() int
	ShouldEscalateRecovery() bool
	MandatoryCommandPrefixes() []string
}

// ConfigValidationPolicy implements ValidationPolicy backed by *config.Config.
type ConfigValidationPolicy struct {
	cfg *config.Config
}

var _ ValidationPolicy = (*ConfigValidationPolicy)(nil)

// NewConfigValidationPolicy returns a ValidationPolicy backed by cfg.
func NewConfigValidationPolicy(cfg *config.Config) ValidationPolicy {
	return &ConfigValidationPolicy{cfg: cfg}
}

// SelectGate returns GateFull when consecutiveSuccesses % fullEveryN == 0 and
// fullEveryN > 0, otherwise GateFast.
func (p *ConfigValidationPolicy) SelectGate(consecutiveSuccesses int) GateType {
	n := p.cfg.Validation.FullValidationEveryN
	if n <= 0 {
		return GateFast
	}
	if consecutiveSuccesses%n == 0 {
		return GateFull
	}
	return GateFast
}

// MaxRecoveryAttempts returns the maximum number of recovery attempts (1 quick
// + 1 escalated), matching the current hardcoded behavior.
func (p *ConfigValidationPolicy) MaxRecoveryAttempts() int {
	return 2
}

// ShouldEscalateRecovery returns true, indicating recovery always escalates to
// a higher-tier model when quick auto-fix fails.
func (p *ConfigValidationPolicy) ShouldEscalateRecovery() bool {
	return true
}

// MandatoryCommandPrefixes returns the list of command prefixes that must
// appear in the validation command set.
func (p *ConfigValidationPolicy) MandatoryCommandPrefixes() []string {
	if p == nil || p.cfg == nil {
		return nil
	}
	if len(p.cfg.Validation.MandatoryCommands) == 0 {
		return copyStrings(p.resolvedValidationCommands())
	}

	prefixes := make([]string, len(p.cfg.Validation.MandatoryCommands))
	copy(prefixes, p.cfg.Validation.MandatoryCommands)
	return prefixes
}

func (p *ConfigValidationPolicy) resolvedValidationCommands() []string {
	if p == nil || p.cfg == nil {
		return nil
	}
	resolvedProfile := p.cfg.ResolvedProfile()
	if resolvedProfile.Source == config.CompatibilitySourceLegacyFallback {
		return nil
	}
	return p.cfg.ResolveProfileDependentDefaults().ValidationCommands
}

func copyStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := append([]string(nil), src...)
	return dst
}
