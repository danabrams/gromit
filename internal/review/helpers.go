package review

// BuildReviewBeadLabels ensures review beads always carry the `from-review` label
// while preserving any additional labels without duplicating the sentinel.
func BuildReviewBeadLabels(proposalLabels []string) []string {
	labels := []string{"from-review"}
	for _, l := range proposalLabels {
		if l == "from-review" {
			continue
		}
		labels = append(labels, l)
	}
	return labels
}

// BuildBacklogLabels returns the canonical label list for backlog items surfaced
// by review results.
func BuildBacklogLabels() []string {
	return []string{"from-review", "backlog"}
}
