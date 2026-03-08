package debug

import "strings"

// BuildSystemicRecommendation produces a human-review recommendation when the
// diagnosed root cause or failure signal implies a system-level change.
func BuildSystemicRecommendation(rootCause RootCause, failureSignal string) string {
	category := detectSystemicCategory(rootCause, failureSignal)
	if category == "" {
		return ""
	}

	recommendation := recommendationForCategory(category)
	trimmedSignal := strings.TrimSpace(failureSignal)
	if trimmedSignal == "" {
		return recommendation
	}
	return recommendation + " Observed signal: " + trimmedSignal
}

func detectSystemicCategory(rootCause RootCause, failureSignal string) string {
	normalized := strings.ToLower(strings.TrimSpace(failureSignal))
	switch {
	case strings.Contains(normalized, "prompt fragment"),
		strings.Contains(normalized, "ambiguous prompt"):
		return "prompt"
	case strings.Contains(normalized, "code guard"),
		strings.Contains(normalized, "pipeline guard"),
		strings.Contains(normalized, "guardrail"):
		return "guard"
	case strings.Contains(normalized, "process change"),
		strings.Contains(normalized, "workflow"),
		strings.Contains(normalized, "decomposition"):
		return "process"
	case strings.Contains(normalized, "rule update"),
		strings.Contains(normalized, "rules.md"):
		return "rule"
	}

	switch rootCause {
	case RootCauseUnclearBead:
		return "prompt"
	case RootCauseBadDecomposition:
		return "process"
	default:
		return ""
	}
}

func recommendationForCategory(category string) string {
	switch category {
	case "prompt":
		return "Systemic recommendation for human review: tighten prompt fragments and acceptance criteria so ambiguous instructions fail early."
	case "guard":
		return "Systemic recommendation for human review: add or strengthen a pipeline code guard so this failure mode is blocked automatically."
	case "process":
		return "Systemic recommendation for human review: update decomposition and validation process checks so broad scopes are split before build attempts."
	case "rule":
		return "Systemic recommendation for human review: add or update a RULES.md policy and enforcement check for this failure class."
	default:
		return ""
	}
}
