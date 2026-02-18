package specgate

import (
	"context"
	"errors"
	"fmt"
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
	}
	label := fmt.Sprintf("spec:%s", specName)
	for i := 0; i < limit; i++ {
		failure := failures[i]
		title := truncateTitle(failure.Criterion, 80)
		id, err := creator.Create(ctx, title, failure.Evidence, priority, []string{label})
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if ids == nil {
		ids = []string{}
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
