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
