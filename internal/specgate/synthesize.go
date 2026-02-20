package specgate

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

const (
	maxSynthesizedFixBeads = 5
)

// BeadCreator creates beads for spec fixes.
type BeadCreator interface {
	Create(ctx context.Context, title, description, priority string, labels []string) (string, error)
}

// SynthesizeFixBeads creates fix beads for failed criteria, capped at five beads.
func SynthesizeFixBeads(ctx context.Context, specName string, failures []CriterionResult, priority string, creator BeadCreator) ([]string, error) {
	if len(failures) == 0 {
		return []string{}, nil
	}
	if creator == nil {
		return nil, errors.New("bead creator is required")
	}

	ids := make([]string, 0, min(len(failures), maxSynthesizedFixBeads))
	limit := len(failures)
	if limit > maxSynthesizedFixBeads {
		limit = maxSynthesizedFixBeads
		log.Printf(
			"specgate: capped fix bead synthesis for spec %q at %d failures (%d remaining)",
			specName,
			maxSynthesizedFixBeads,
			len(failures)-limit,
		)
	}
	label := fmt.Sprintf("spec:%s", specName)
	createErrors := make([]error, 0, limit)
	for i := 0; i < limit; i++ {
		failure := failures[i]
		title := formatFailureTitle(failure)
		description := formatFailureDescription(failure)
		id, err := creator.Create(ctx, title, description, priority, []string{label})
		if err != nil {
			createErrors = append(createErrors, fmt.Errorf("create bead for criterion %q: %w", failure.Criterion, err))
			continue
		}
		ids = append(ids, id)
	}
	if len(createErrors) > 0 {
		return ids, errors.Join(createErrors...)
	}
	return ids, nil
}

func formatFailureTitle(failure CriterionResult) string {
	criterion := strings.TrimSpace(failure.Criterion)
	if criterion == "" {
		criterion = "unnamed criterion"
	}
	return fmt.Sprintf("Fix: %s", criterion)
}

func formatFailureDescription(failure CriterionResult) string {
	criterion := strings.TrimSpace(failure.Criterion)
	if criterion == "" {
		criterion = "unnamed criterion"
	}

	parts := []string{
		fmt.Sprintf("Criterion: %s", criterion),
		fmt.Sprintf("Evidence: %s", strings.TrimSpace(failure.Evidence)),
		fmt.Sprintf("Fix direction: Implement changes so '%s' passes in verify-spec.", criterion),
	}
	return strings.Join(parts, "\n")
}
