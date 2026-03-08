package gate

// satisfactionTier returns the LLM tier for the pre-build satisfaction check
// based on bead generation. Gen 0 returns "" (skip). Gen 1 = low (haiku),
// gen 2 = medium (sonnet), gen 3+ = high (opus).
func satisfactionTier(generation int) string {
	switch {
	case generation <= 0:
		return ""
	case generation == 1:
		return "low"
	case generation == 2:
		return "medium"
	default:
		return "high"
	}
}
