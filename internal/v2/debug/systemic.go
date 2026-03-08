package debug

import (
	"fmt"
	"strings"
)

// BuildSystemicRecommendation produces a human-review recommendation when the
// diagnosed root cause or failure signal implies a system-level change.
func BuildSystemicRecommendation(rootCause RootCause, failureSignal string) string {
	category := DetectSystemicCategory(rootCause, failureSignal)
	if category == "" {
		return ""
	}

	change := changeDescriptionForCategory(category)
	if change == "" {
		return ""
	}

	rationale := buildSystemicRationale(rootCause, failureSignal, category)
	if rationale == "" {
		rationale = fmt.Sprintf("Systemic category %q requires human-reviewed changes before the pipeline can proceed.", category)
	}

	return fmt.Sprintf(
		"Systemic recommendation for human review: %s Rationale: %s Awaiting human approval before applying these changes.",
		change,
		rationale,
	)
}

// DetectSystemicCategory determines whether the root cause or failure signal
// maps to a systemic change category that should trigger human review.
func DetectSystemicCategory(rootCause RootCause, failureSignal string) string {
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

func changeDescriptionForCategory(category string) string {
	switch category {
	case "prompt":
		return "Tighten prompt fragments and acceptance criteria so ambiguous instructions fail early."
	case "guard":
		return "Add or strengthen a pipeline code guard so this failure mode is blocked automatically."
	case "process":
		return "Update decomposition and validation process checks so broad scopes are split before build attempts."
	case "rule":
		return "Add or update a RULES.md policy and enforcement check for this failure class."
	default:
		return ""
	}
}

func buildSystemicRationale(rootCause RootCause, failureSignal, category string) string {
	if trimmed := strings.TrimSpace(failureSignal); trimmed != "" {
		return trimmed
	}

	switch category {
	case "prompt":
		return fmt.Sprintf("Root cause %q reflects an unclear bead description or prompt fragment that must be clarified before rerunning.", rootCause)
	case "guard":
		return fmt.Sprintf("Root cause %q indicates this change bypassed existing guardrails, so pipeline checks must stop it before retries.", rootCause)
	case "process":
		return fmt.Sprintf("Root cause %q signals a decomposition or validation gap that needs tighter process controls before build stages.", rootCause)
	case "rule":
		return "Policy enforcement around this workflow is missing or outdated; capture the expectation in RULES.md before continuing."
	default:
		return ""
	}
}
