package benchmark

import (
	"fmt"
	stdstrings "strings"

	"github.com/danabrams/gromit/internal/bead"
)

type BeadLookup interface {
	Show(id string) (*bead.Bead, error)
}

func ValidateSelectedCohort(lookup BeadLookup, selected []string, requiredSize int, requireTierCoverage bool) ([]string, error) {
	if len(selected) < requiredSize {
		return nil, fmt.Errorf("selected cohort size %d is below minimum %d", len(selected), requiredSize)
	}
	if len(selected) > requiredSize {
		return nil, fmt.Errorf("selected cohort size %d exceeds required %d", len(selected), requiredSize)
	}

	var covered map[string]bool
	if requireTierCoverage {
		covered = map[string]bool{
			"low":    false,
			"medium": false,
			"high":   false,
		}
	}

	for _, id := range selected {
		b, err := lookup.Show(id)
		if err != nil {
			return nil, fmt.Errorf("selected bead %q lookup failed: %w", id, err)
		}
		if b.Status != "open" {
			return nil, fmt.Errorf("selected bead %q must be open, got %q", id, b.Status)
		}
		if requireTierCoverage {
			tier, err := parseComplexityTier(b.Labels)
			if err != nil {
				return nil, fmt.Errorf("selected bead %q: %w", id, err)
			}
			covered[tier] = true
		}
	}

	if requireTierCoverage {
		missing := make([]string, 0, 3)
		for _, tier := range []string{"low", "medium", "high"} {
			if !covered[tier] {
				missing = append(missing, tier)
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("missing complexity tiers: %s", stdstrings.Join(missing, ","))
		}
	}

	return append([]string(nil), selected...), nil
}

func complexityTier(labels []string) string {
	tier, err := parseComplexityTier(labels)
	if err != nil {
		return "medium"
	}
	return tier
}

func parseComplexityTier(labels []string) (string, error) {
	for _, label := range labels {
		switch stdstrings.TrimSpace(stdstrings.ToLower(label)) {
		case "complexity:low":
			return "low", nil
		case "complexity:medium":
			return "medium", nil
		case "complexity:high":
			return "high", nil
		default:
			if stdstrings.HasPrefix(stdstrings.TrimSpace(stdstrings.ToLower(label)), "complexity:") {
				return "", fmt.Errorf("unsupported complexity label %q", label)
			}
		}
	}
	return "medium", nil
}
