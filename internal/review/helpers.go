package review

import "strings"

// ExpectedOutputsOrTitle returns expected outputs when provided or falls back to a trimmed title.
func ExpectedOutputsOrTitle(outputs []string, title string) []string {
	if len(outputs) > 0 {
		return outputs
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return []string{}
	}
	return []string{trimmedTitle}
}

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
