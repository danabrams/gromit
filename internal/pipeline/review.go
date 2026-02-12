package pipeline

// buildThoroughReviewContext constructs the context for rendering the review prompt
func buildThoroughReviewContext(input ReviewInput) map[string]interface{} {
	return map[string]interface{}{
		"Diff":  input.Diff,
		"Model": input.Model,
	}
}

// buildReviewBeadLabels adds the from-review label to existing labels
func buildReviewBeadLabels(proposalLabels []string) []string {
	labels := []string{"from-review"}
	for _, l := range proposalLabels {
		if l != "from-review" {
			labels = append(labels, l)
		}
	}
	return labels
}
