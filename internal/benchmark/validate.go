package benchmark

import (
	"fmt"
	stdstrings "strings"

	"github.com/danabrams/gromit/internal/bead"
)

type BeadLookup interface {
	Show(id string) (*bead.Bead, error)
}

func ValidateSelectedCohort(lookup BeadLookup, selected []string, minSize int) ([]string, error) {
	if len(selected) < minSize {
		return nil, fmt.Errorf("selected cohort size %d is below minimum %d", len(selected), minSize)
	}

	for _, id := range selected {
		b, err := lookup.Show(id)
		if err != nil {
			return nil, fmt.Errorf("selected bead %q lookup failed: %w", id, err)
		}
		if b.Status != "open" {
			return nil, fmt.Errorf("selected bead %q must be open, got %q", id, b.Status)
		}
	}

	return append([]string(nil), selected...), nil
}

func complexityTier(labels []string) string {
	for _, label := range labels {
		switch stdstrings.TrimSpace(stdstrings.ToLower(label)) {
		case "complexity:low":
			return "low"
		case "complexity:medium":
			return "medium"
		case "complexity:high":
			return "high"
		}
	}
	return "medium"
}
