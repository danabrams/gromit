package routing

// ResolveModel maps an abstract tier name to a provider-specific model name.
// Uses the provider's tier-to-model map (e.g. {"low": "claude-haiku-4-5-20251001"}).
// Returns tier as-is if no mapping is found.
func ResolveModel(tier string, models map[string]string) string {
	if model, ok := models[tier]; ok && model != "" {
		return model
	}
	return tier
}

// TierForPhase returns the effective tier for a phase.
// phaseModels maps phase name to tier name (from methodology.phase_models config).
// Falls back to fallbackTier when the phase has no configured override.
func TierForPhase(phase string, phaseModels map[string]string, fallbackTier string) string {
	if tier, ok := phaseModels[phase]; ok && tier != "" {
		return tier
	}
	return fallbackTier
}
