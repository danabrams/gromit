package benchmark

import (
	"fmt"

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
