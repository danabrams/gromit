package debug

// BuildSystemicRecommendation produces a human-review recommendation when the
// diagnosed root cause implies a system-level change.
func BuildSystemicRecommendation(rootCause RootCause) string {
	switch rootCause {
	case RootCauseUnclearBead:
		return "Systemic recommendation for human review: clarify prompt fragments and acceptance criteria so ambiguous bead descriptions are rejected earlier."
	case RootCauseBadDecomposition:
		return "Systemic recommendation for human review: tighten decomposition process checks so broad scopes are split before build attempts."
	default:
		return ""
	}
}
