package andon

import "time"

// DefaultThresholds returns the default policy bounds from the Andon spec.
func DefaultThresholds() AndonThresholds {
	return AndonThresholds{
		L1MaxRetries:   2,
		L1MaxDuration:  2 * time.Minute,
		L2MaxDuration:  15 * time.Minute,
		MaxAssumptions: 2,
	}
}

// ClassifyFailure maps an observed signal to an Andon failure class.
func ClassifyFailure(signal FailureSignal) FailureClass {
	switch signal.Kind {
	case FailureKindTimeout:
		return FailureClassTransient
	case FailureKindWorkflow:
		return FailureClassWorkflow
	case FailureKindQualityGate:
		return FailureClassQuality
	case FailureKindAmbiguousIntent:
		return FailureClassIntent
	case FailureKindIntegrity:
		return FailureClassData
	default:
		return FailureClassWorkflow
	}
}

// ChooseNextAction computes the next bounded recovery step for a failure state.
func ChooseNextAction(state RecoveryState, thresholds AndonThresholds, now time.Time) PolicyDecision {
	if state.Class == FailureClassData {
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine}
	}

	if state.Class == FailureClassIntent && state.AssumptionsUsed >= thresholds.MaxAssumptions {
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate}
	}

	if state.Class == FailureClassWorkflow && state.Level == LevelL1 && state.L1Attempts >= 1 {
		return PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}
	}

	if state.Level == LevelL2 {
		if elapsed(state.L2Started, now) > thresholds.L2MaxDuration {
			return PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine}
		}

		return PolicyDecision{NextLevel: LevelL2, Action: DecisionRetry}
	}

	if state.L1Attempts >= thresholds.L1MaxRetries || elapsed(state.L1Started, now) > thresholds.L1MaxDuration {
		return PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}
	}

	return PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry}
}

func elapsed(start, now time.Time) time.Duration {
	if start.IsZero() || now.Before(start) {
		return 0
	}

	return now.Sub(start)
}
