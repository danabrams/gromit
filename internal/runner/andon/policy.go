package andon

import "time"

// DefaultThresholds returns the default policy bounds from the Andon spec.
func DefaultThresholds() AndonThresholds {
	return DefaultThresholdDefinition()
}

// PolicyInput is the deterministic policy input without command or clock dependencies.
type PolicyInput struct {
	State      RecoveryState
	Thresholds AndonThresholds
	L1Elapsed  time.Duration
	L2Elapsed  time.Duration
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
	return ChooseNextActionPure(PolicyInput{
		State:      state,
		Thresholds: thresholds,
		L1Elapsed:  elapsed(state.L1Started, now),
		L2Elapsed:  elapsed(state.L2Started, now),
	})
}

// ChooseNextActionPure computes the next bounded recovery step for a failure state.
func ChooseNextActionPure(input PolicyInput) PolicyDecision {
	if input.State.Class == FailureClassData {
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine}
	}

	if input.State.Class == FailureClassIntent && input.State.AssumptionsUsed >= input.Thresholds.MaxAssumptions {
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate}
	}

	if input.State.Class == FailureClassWorkflow && input.State.Level == LevelL1 && input.State.L1Attempts >= 1 {
		return PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}
	}

	if input.State.Level == LevelL2 {
		if input.L2Elapsed >= input.Thresholds.L2MaxDuration {
			return PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine}
		}

		return PolicyDecision{NextLevel: LevelL2, Action: DecisionRetry}
	}

	if input.State.L1Attempts >= input.Thresholds.L1MaxRetries || input.L1Elapsed >= input.Thresholds.L1MaxDuration {
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
