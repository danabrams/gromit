package reviewpacket

// ComputeTrustLevel computes the trust level from validation, acceptance, review, and degraded signals.
//
// Rules:
// - high: validation passed AND all acceptance passed AND no blocking findings AND no degraded flags AND no repeated-failure escalation
// - medium: validation passed AND no blockers AND (degraded flags OR non-blocking concerns)
// - low: blocked/needs_human state OR validation incomplete OR blocking issues OR repeated-failure escalation
func ComputeTrustLevel(
	terminalState string,
	validationPassed bool,
	acceptanceAllPassed bool,
	hasBlockingFindings bool,
	hasDegradedFlags bool,
	repeatedFailureEscalated bool,
) string {
	// low: if terminal state is blocked or needs_human
	if terminalState == "blocked" || terminalState == "needs_human" {
		return "low"
	}

	// low: if validation failed
	if !validationPassed {
		return "low"
	}

	// low: if acceptance criteria not all passed
	if !acceptanceAllPassed {
		return "low"
	}

	// low: if there are blocking findings
	if hasBlockingFindings {
		return "low"
	}

	// low: if repeated failure escalation fired
	if repeatedFailureEscalated {
		return "low"
	}

	// high: validation passed AND all acceptance passed AND no blockers AND no degraded flags AND no repeated failure
	if !hasDegradedFlags {
		return "high"
	}

	// medium: all checks passed but degraded flags are present
	return "medium"
}

// RecommendedPosture derives the recommended review posture from the trust level.
func RecommendedPosture(trustLevel string) string {
	switch trustLevel {
	case "high":
		return "quick_accept_path"
	case "medium":
		return "manual_check_carefully"
	case "low":
		return "do_not_accept_without_changes"
	default:
		return ""
	}
}
