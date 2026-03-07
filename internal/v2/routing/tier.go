package routing

// TierForPhase returns the effective tier for a phase.
// phaseModels maps phase name to tier name (from methodology.phase_models config).
// Falls back to fallbackTier when the phase has no configured override.
func TierForPhase(phase string, phaseModels map[string]string, fallbackTier string) string {
	if tier, ok := phaseModels[phase]; ok && tier != "" {
		return tier
	}
	return fallbackTier
}
