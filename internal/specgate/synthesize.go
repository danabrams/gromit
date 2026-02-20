package specgate

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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

	var ids []string
	limit := len(failures)
	if limit > 5 {
		limit = 5
		log.Printf("specgate: capped fix bead synthesis for spec %q at 5 failures (%d remaining)", specName, len(failures)-limit)
	}
	label := fmt.Sprintf("spec:%s", specName)
	createErrors := make([]error, 0)
	for i := 0; i < limit; i++ {
		failure := failures[i]
		title := truncateTitle(failure.Criterion, 80)
		description := formatFailureDescription(failure)
		id, err := creator.Create(ctx, title, description, priority, []string{label})
		if err != nil {
			createErrors = append(createErrors, fmt.Errorf("create bead for criterion %q: %w", failure.Criterion, err))
			continue
		}
		ids = append(ids, id)
	}
	if ids == nil {
		ids = []string{}
	}
	if len(createErrors) > 0 {
		return ids, errors.Join(createErrors...)
	}
	return ids, nil
}

func truncateTitle(title string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(title)
	if len(runes) <= max {
		return title
	}
	return string(runes[:max])
}

func formatFailureDescription(failure CriterionResult) string {
	parts := []string{
		fmt.Sprintf("Criterion: %s", strings.TrimSpace(failure.Criterion)),
		fmt.Sprintf("Evidence: %s", strings.TrimSpace(failure.Evidence)),
	}
	return strings.Join(parts, "\n")
}
