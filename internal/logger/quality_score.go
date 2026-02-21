package logger

const (
	qualityPenaltyValidationRetried = 0.10
	qualityPenaltyTrivialAutoFixed  = 0.10
	qualityPenaltyEscalated         = 0.15
	qualityPenaltyPerReviewFix      = 0.05
	qualityPenaltyReviewFixCap      = 0.20
)

// ComputeQualityScore returns a 0.0-1.0 score for iteration quality.
func ComputeQualityScore(criteriaTotal, criteriaCovered int, validationRetried, trivialAutoFixed, escalated bool, reviewFixesNeeded int) float64 {
	baseCoverage := 1.0
	if criteriaTotal > 0 {
		baseCoverage = float64(criteriaCovered) / float64(criteriaTotal)
	}
	baseCoverage = clamp(baseCoverage, 0, 1)

	penalty := 0.0
	if validationRetried {
		penalty += qualityPenaltyValidationRetried
	}
	if trivialAutoFixed {
		penalty += qualityPenaltyTrivialAutoFixed
	}
	if escalated {
		penalty += qualityPenaltyEscalated
	}
	if reviewFixesNeeded > 0 {
		reviewPenalty := float64(reviewFixesNeeded) * qualityPenaltyPerReviewFix
		if reviewPenalty > qualityPenaltyReviewFixCap {
			reviewPenalty = qualityPenaltyReviewFixCap
		}
		penalty += reviewPenalty
	}

	return clamp(baseCoverage-penalty, 0, 1)
}
