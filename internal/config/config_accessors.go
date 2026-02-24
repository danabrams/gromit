package config

import (
	"strings"
	"time"
)

const (
	tierHigh   = "high"
	tierMedium = "medium"
	tierLow    = "low"
)

var legacyModelToTier = map[string]string{
	"opus":          tierHigh,
	"sonnet":        tierMedium,
	"haiku":         tierLow,
	"o3":            tierHigh,
	"gpt-4o":        tierMedium,
	"gpt-4o-mini":   tierLow,
	"gpt-5.3-codex": tierMedium,
	"gpt-5-mini":    tierLow,
}

func (f FallbackConfig) EnabledOrDefault(multiProvider bool) bool {
	if f.Enabled == nil {
		return multiProvider
	}
	return *f.Enabled
}

func (s StreamConfig) PreserveProviderOutputEnabled() bool {
	if s.PreserveProviderOutput == nil {
		return true
	}
	return *s.PreserveProviderOutput
}

func (v ValidationConfig) IsNonInteractive() bool {
	if v.NonInteractive == nil {
		return true
	}
	return *v.NonInteractive
}

// FastCommandsOrDefault returns the per-bead validation command set.
// `fast_commands` takes precedence, then falls back to legacy `commands`.
func (v ValidationConfig) FastCommandsOrDefault() []string {
	if len(v.FastCommands) > 0 {
		return v.FastCommands
	}
	return v.Commands
}

// FullCommandsOrDefault returns the full verification command set.
// `full_commands` takes precedence, then falls back to legacy `commands`.
func (v ValidationConfig) FullCommandsOrDefault() []string {
	if len(v.FullCommands) > 0 {
		return v.FullCommands
	}
	return v.Commands
}

// ShouldRunFinalFullGate returns whether the session-ending full validation
// gate should run (defaults to true).
func (v ValidationConfig) ShouldRunFinalFullGate() bool {
	if v.RunFinalFullGate == nil {
		return true
	}
	return *v.RunFinalFullGate
}

// ResolvePhaseTimeoutSeconds returns validation's phase timeout override, or
// beadTimeoutSeconds when phase_timeout_seconds is unset/zero.
func (v ValidationConfig) ResolvePhaseTimeoutSeconds(beadTimeoutSeconds int) int {
	if v.PhaseTimeoutSeconds > 0 {
		return v.PhaseTimeoutSeconds
	}
	return beadTimeoutSeconds
}

// PlanMaxSubBeadsValue returns the configured plan decomposition max sub-bead
// count. A value <= 0 means the plan max-size check is disabled.
func (v ValidationConfig) PlanMaxSubBeadsValue() int {
	if v.PlanMaxSubBeads == nil {
		return DefaultMaxSubBeads
	}
	return *v.PlanMaxSubBeads
}

// RuntimeMaxSubBeadsValue returns the configured runtime decomposition max
// sub-bead count.
func (v ValidationConfig) RuntimeMaxSubBeadsValue() int {
	if v.RuntimeMaxSubBeads <= 0 {
		return DefaultMaxSubBeads
	}
	return v.RuntimeMaxSubBeads
}

// IsTierName returns true if the string is a valid tier name (high, medium, low).
// Case-insensitive comparison handles config variations like "High", "MEDIUM", etc.
func (c *Config) IsTierName(s string) bool {
	switch strings.ToLower(s) {
	case "high", "medium", "low":
		return true
	default:
		return false
	}
}

// SelectTier determines the appropriate tier for a bead based on priority and labels.
// Returns abstract tier names (high, medium, low) instead of provider-specific model names.
// For backward compatibility, auto-maps legacy model names via TierFromLegacyModel when
// config contains model names instead of tier names.
func (c *Config) SelectTier(priority int, labels []string) string {
	if c == nil {
		return tierMedium
	}

	// Check label overrides first (higher precedence)
	for _, label := range labels {
		if value, ok := c.Models.Labels[label]; ok {
			// Auto-map legacy model names to tiers
			return tierFromLegacyModel(value)
		}
	}

	// Fall back to priority-based selection
	var value string
	switch priority {
	case 0:
		value = c.Models.P0
	case 1:
		value = c.Models.P1
	case 2:
		value = c.Models.P2
	default:
		value = c.Models.P1 // Default to medium tier for unknown priorities
	}

	// Auto-map legacy model names to tiers
	return tierFromLegacyModel(value)
}

// SelectInitialTierForComplexity maps effective complexity to an initial tier.
func (c *Config) SelectInitialTierForComplexity(complexity string) string {
	normalized := strings.ToLower(strings.TrimSpace(complexity))
	switch normalized {
	case tierLow:
		return tierLow
	case tierHigh:
		return tierHigh
	}
	if mapped := tierFromLegacyModel(normalized); c.IsTierName(mapped) {
		return mapped
	}
	return tierMedium
}

// SelectModel determines the appropriate model for a bead based on priority and labels.
func (c *Config) SelectModel(priority int, labels []string) string {
	if c == nil {
		return ModelSonnet
	}
	for _, label := range labels {
		if model, ok := c.Models.Labels[label]; ok {
			return model
		}
	}

	switch priority {
	case 0:
		return c.Models.P0
	case 1:
		return c.Models.P1
	case 2:
		return c.Models.P2
	default:
		return c.Models.P1 // Default to sonnet for unknown priorities
	}
}

// NextEscalationTier returns the next tier in the escalation chain, or empty if at end.
// For backward compatibility, auto-maps legacy model names in the chain and input to tiers.
func (c *Config) NextEscalationTier(currentTier string) string {
	if c == nil {
		return ""
	}
	if !c.Escalation.Enabled {
		return ""
	}

	mappedCurrentTier := tierFromLegacyModel(currentTier)

	for i, chainEntry := range c.Escalation.Chain {
		mappedChainEntry := tierFromLegacyModel(chainEntry)

		if mappedChainEntry == mappedCurrentTier && i+1 < len(c.Escalation.Chain) {
			return tierFromLegacyModel(c.Escalation.Chain[i+1])
		}
	}
	return ""
}

func tierFromLegacyModel(modelName string) string {
	if tier, ok := legacyModelToTier[strings.ToLower(modelName)]; ok {
		return tier
	}
	return modelName
}

// NextEscalationModel returns the next model in the escalation chain, or empty if at end.
func (c *Config) NextEscalationModel(currentModel string) string {
	if c == nil {
		return ""
	}
	if !c.Escalation.Enabled {
		return ""
	}

	for i, model := range c.Escalation.Chain {
		if model == currentModel && i+1 < len(c.Escalation.Chain) {
			return c.Escalation.Chain[i+1]
		}
	}
	return ""
}

// ShouldMatchBuildModel returns whether review should use the same model as build (opus only).
func (r ReviewConfig) ShouldMatchBuildModel() bool {
	if r.MatchBuildModel == nil {
		return true
	}
	return *r.MatchBuildModel
}

// ShouldRunOnEpicComplete returns whether thorough review should run when an epic completes.
func (t ThoroughReviewConfig) ShouldRunOnEpicComplete() bool {
	if t.OnEpicComplete == nil {
		return true
	}
	return *t.OnEpicComplete
}

// ShouldLearnFromSuccess returns whether to extract learnings from successful iterations.
func (l LoopConfig) ShouldLearnFromSuccess() bool {
	if l.LearnFromSuccess == nil {
		return true
	}
	return *l.LearnFromSuccess
}

// PhaseModelTier returns a phase-specific tier override when configured.
// Unknown phases and empty overrides fall back to beadTier.
func (c *Config) PhaseModelTier(phase, beadTier string) string {
	if c == nil {
		return beadTier
	}

	var override string
	switch strings.ToLower(phase) {
	case "decompose":
		override = c.Methodology.PhaseModels.Decompose
	case "build":
		override = c.Methodology.PhaseModels.Build
	case "red":
		override = c.Methodology.PhaseModels.Red
	case "green":
		override = c.Methodology.PhaseModels.Green
	case "refactor":
		override = c.Methodology.PhaseModels.Refactor
	default:
		return beadTier
	}

	if override != "" {
		return override
	}
	return beadTier
}

// IsVerificationEnabled returns whether precheck verification should run (defaults to true).
func (v PrecheckVerificationConfig) IsVerificationEnabled() bool {
	if v.Enabled == nil {
		return true
	}
	return *v.Enabled
}

// IsEnabled returns whether precheck should run (defaults to false).
// Changed from default-true to default-false due to persistent false-positive
// closures (see .gromit/reports/debug-20260219-153000.md).
func (p PrecheckConfig) IsEnabled() bool {
	if p.Enabled == nil {
		return false
	}
	return *p.Enabled
}

// IsAutoPushEnabled returns whether git auto-push should run after bead completion (defaults to true).
func (g GitConfig) IsAutoPushEnabled() bool {
	if g.AutoPush == nil {
		return true
	}
	return *g.AutoPush
}

// PushTimeoutDuration returns the git push timeout duration (0 disables the timeout).
func (g GitConfig) PushTimeoutDuration() time.Duration {
	if g.PushTimeout == 0 {
		return 0
	}
	return time.Duration(g.PushTimeout) * time.Second
}

// TimeoutsForModel returns the effective invocation, stall, stall-active, and bead timeouts for a model.
// Per-model overrides (if non-zero) take precedence over the top-level defaults.
func (c ClaudeConfig) TimeoutsForModel(model string) (invocationTimeout, stallTimeout, stallTimeoutActive, beadTimeout int) {
	invocationTimeout = c.Timeout
	stallTimeout = c.StallTimeout
	stallTimeoutActive = c.StallTimeoutActive
	beadTimeout = c.BeadTimeout

	if overrides, ok := c.ModelTimeouts[model]; ok {
		if overrides.Timeout > 0 {
			invocationTimeout = overrides.Timeout
		}
		if overrides.StallTimeout > 0 {
			stallTimeout = overrides.StallTimeout
		}
		if overrides.StallTimeoutActive > 0 {
			stallTimeoutActive = overrides.StallTimeoutActive
		}
		if overrides.BeadTimeout > 0 {
			beadTimeout = overrides.BeadTimeout
		}
	}
	return
}

// TokenBudgetForModel returns the effective max input tokens budget for a model.
// A non-zero per-model override takes precedence over the top-level default.
func (c ClaudeConfig) TokenBudgetForModel(model string) int {
	tokenBudget := c.MaxInputTokensPerBead

	if overrides, ok := c.ModelTimeouts[model]; ok && overrides.MaxInputTokensPerBead > 0 {
		tokenBudget = overrides.MaxInputTokensPerBead
	}

	return tokenBudget
}

// ShouldBlockOversized returns whether over-scoped beads should be blocked before execution (defaults to true).
func (s ScopeCheckConfig) ShouldBlockOversized() bool {
	if s.BlockOversized == nil {
		return true
	}
	return *s.BlockOversized
}

// HasProviders returns true when providers section is non-empty.
func (c *Config) HasProviders() bool {
	if c == nil {
		return false
	}
	return len(c.Providers) > 0
}

// IsEnabled returns whether worktree isolation is enabled (defaults to true).
func (w WorktreeConfig) IsEnabled() bool {
	if w.Enabled == nil {
		return true
	}
	return *w.Enabled
}

// IsAutoMergeEnabled returns whether automatic merge-back is enabled (defaults to true).
func (w WorktreeConfig) IsAutoMergeEnabled() bool {
	if w.AutoMerge == nil {
		return true
	}
	return *w.AutoMerge
}

// IsEnabled returns whether spec gate is enabled (defaults to true).
func (s SpecGateConfig) IsEnabled() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

// IsAutoTrigger returns whether spec gate auto-trigger is enabled (defaults to true).
func (s SpecGateConfig) IsAutoTrigger() bool {
	if s.AutoTrigger == nil {
		return true
	}
	return *s.AutoTrigger
}

// ResolvedMethodologyAdapter returns the resolved methodology adapter selector
// and source metadata from compatibility resolution.
func (c Config) ResolvedMethodologyAdapter() CompatibilityResolvedValue {
	return c.ResolveCompatibilityContext().MethodologyAdapter
}
