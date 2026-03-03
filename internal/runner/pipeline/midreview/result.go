package midreview

// Finding represents a single finding from the mid-build review.
type Finding struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

// MidBuildReviewResult represents the aggregated result of a mid-build review.
type MidBuildReviewResult struct {
	Findings []Finding `json:"findings"`
	Summary  string    `json:"summary"`
}
