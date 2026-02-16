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
