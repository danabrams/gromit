package experiment

import (
	"math/rand"
)

// ArmState represents the state of a single arm in the Thompson sampling bandit.
type ArmState struct {
	ID        string
	Successes int
	Failures  int
}

// BanditState represents the state of a multi-arm bandit.
type BanditState struct {
	Arms []ArmState
}

// SelectVariant selects a variant using Thompson sampling.
// If forceVariant is non-empty, returns that variant.
// Otherwise, uses Thompson sampling with Monte Carlo simulation over 10,000 draws.
func (bs *BanditState) SelectVariant(forceVariant string) string {
	if forceVariant != "" {
		return forceVariant
	}

	if len(bs.Arms) == 0 {
		return ""
	}

	// Thompson sampling: sample from Beta distribution for each arm
	// and select the one with highest sampled value
	const numDraws = 10000

	armMeans := make([]float64, len(bs.Arms))
	for i, arm := range bs.Arms {
		// Monte Carlo estimation: draw from Beta(successes+1, failures+1)
		// and compute mean across 10,000 draws
		sum := 0.0
		for draw := 0; draw < numDraws; draw++ {
			// Simple Beta sampling: use exponential method
			// X ~ Gamma(a, 1), Y ~ Gamma(b, 1), then X/(X+Y) ~ Beta(a, b)
			x := sampleGamma(float64(arm.Successes + 1))
			y := sampleGamma(float64(arm.Failures + 1))
			sum += x / (x + y)
		}
		armMeans[i] = sum / numDraws
	}

	// Select arm with highest mean
	bestIdx := 0
	bestMean := armMeans[0]
	for i, mean := range armMeans {
		if mean > bestMean {
			bestMean = mean
			bestIdx = i
		}
	}

	return bs.Arms[bestIdx].ID
}

// RecordOutcome records the outcome of selecting an arm.
// success=true increments successes, success=false increments failures.
func (bs *BanditState) RecordOutcome(armID string, success bool) {
	for i, arm := range bs.Arms {
		if arm.ID == armID {
			if success {
				bs.Arms[i].Successes++
			} else {
				bs.Arms[i].Failures++
			}
			break
		}
	}
}

// IsConverged determines if the bandit has converged with high confidence.
// Uses Monte Carlo simulation over 10,000 draws to estimate convergence.
// Returns true if the best arm is identified with >= confidenceThreshold probability.
func (bs *BanditState) IsConverged(confidenceThreshold float64) bool {
	if len(bs.Arms) == 0 {
		return true
	}

	if len(bs.Arms) == 1 {
		return true
	}

	const numDraws = 10000

	// Count how many times each arm is the best across draws
	bestArmCounts := make([]int, len(bs.Arms))

	for draw := 0; draw < numDraws; draw++ {
		// Sample from Beta distribution for each arm
		maxValue := -1.0
		bestIdx := -1

		for i, arm := range bs.Arms {
			// Sample from Beta(successes+1, failures+1)
			x := sampleGamma(float64(arm.Successes + 1))
			y := sampleGamma(float64(arm.Failures + 1))
			sample := x / (x + y)

			if sample > maxValue {
				maxValue = sample
				bestIdx = i
			}
		}

		if bestIdx >= 0 {
			bestArmCounts[bestIdx]++
		}
	}

	// Find the arm that was best most frequently
	maxCount := 0
	for _, count := range bestArmCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	// Check if the best arm's confidence is above threshold
	confidence := float64(maxCount) / numDraws
	return confidence >= confidenceThreshold
}

// sampleGamma samples from Gamma(shape, 1) distribution using Marsaglia-Tsang method
func sampleGamma(shape float64) float64 {
	if shape < 1 {
		return sampleGamma(shape+1) * rand.Float64() * rand.Float64()
	}

	d := shape - 1.0/3.0
	c := 1.0 / (9.0 * d)

	for {
		var z float64
		for {
			x := rand.NormFloat64()
			v := 1 + c*x
			if v > 0 {
				z = x * x * x
				break
			}
		}

		u := rand.Float64()
		threshold := 1 - 0.0331*(z*z)*(z*z)
		if u < threshold {
			return d * (1 + c*z)
		}

		if d*(1+c*z) <= 0 {
			continue
		}

		if rand.Float64() < (0.5 * z) {
			continue
		}

		return d * (1 + c*z)
	}
}
