package pipeline

// BuildFromReviewLabels prepends "from-review" to labels, deduplicating if already present.
func BuildFromReviewLabels(proposalLabels []string) []string {
	labels := []string{"from-review"}
	for _, l := range proposalLabels {
		if l != "from-review" {
			labels = append(labels, l)
		}
	}
	return labels
}

// buildReviewBeadLabels adds the from-review label to existing labels
// Deprecated: use BuildFromReviewLabels instead
func buildReviewBeadLabels(proposalLabels []string) []string {
	return BuildFromReviewLabels(proposalLabels)
}
