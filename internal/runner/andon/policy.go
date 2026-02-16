package andon

import "time"

const workflowEscalationAttemptThreshold = 1

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

// ClassifyFailureEntry classifies a failure signal for policy-level decisioning.
func ClassifyFailureEntry(signal FailureSignal) PolicyClassification {
	_ = signal

	return PolicyClassification{Class: FailureClassWorkflow}
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

// EvaluateFailure classifies a failure signal and selects the next action.
func EvaluateFailure(signal FailureSignal, state RecoveryState, thresholds AndonThresholds, now time.Time) PolicyEvaluation {
	class := ClassifyFailure(signal)
	state.Class = class

	return PolicyEvaluation{
		Class:    class,
		Decision: ChooseNextAction(state, thresholds, now),
	}
}

// ChooseNextActionPure computes the next bounded recovery step for a failure state.
func ChooseNextActionPure(input PolicyInput) PolicyDecision {
	thresholds := normalizeThresholds(input.Thresholds)
	state := input.State

	if state.Class == FailureClassData {
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine}
	}

	if state.Class == FailureClassIntent && state.AssumptionsUsed >= thresholds.MaxAssumptions {
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate}
	}

	if state.Class == FailureClassWorkflow &&
		state.Level == LevelL1 &&
		state.L1Attempts >= workflowEscalationAttemptThreshold {
		return PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}
	}

	if state.Level == LevelL2 {
		if input.L2Elapsed >= thresholds.L2MaxDuration {
			return PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine}
		}

		return PolicyDecision{NextLevel: LevelL2, Action: DecisionRetry}
	}

	if state.L1Attempts >= thresholds.L1MaxRetries || input.L1Elapsed >= thresholds.L1MaxDuration {
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

func normalizeThresholds(thresholds AndonThresholds) AndonThresholds {
	defaults := DefaultThresholdDefinition()

	if thresholds.L1MaxRetries <= 0 {
		thresholds.L1MaxRetries = defaults.L1MaxRetries
	}
	if thresholds.L1MaxDuration <= 0 {
		thresholds.L1MaxDuration = defaults.L1MaxDuration
	}
	if thresholds.L2MaxDuration <= 0 {
		thresholds.L2MaxDuration = defaults.L2MaxDuration
	}
	if thresholds.MaxAssumptions <= 0 {
		thresholds.MaxAssumptions = defaults.MaxAssumptions
	}

	return thresholds
}
