package runner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

const (
	maxSynthesizedSpecFixBeads = 5
	defaultGateFailureName     = "unnamed gate failure"
)

// SynthesizeFixBeads creates up to five P0 fix beads for gate failures.
func SynthesizeFixBeads(ctx context.Context, specName string, failures []GateFailure, beadClient BeadClient) ([]string, error) {
	if len(failures) == 0 {
		return []string{}, nil
	}
	if beadClient == nil {
		return nil, errors.New("bead client is required")
	}

	limit := len(failures)
	if limit > maxSynthesizedSpecFixBeads {
		limit = maxSynthesizedSpecFixBeads
		log.Printf(
			"runner: capped fix bead synthesis for spec %q at %d failures (%d remaining)",
			specName,
			maxSynthesizedSpecFixBeads,
			len(failures)-limit,
		)
	}

	ids := make([]string, 0, limit)
	createErrors := make([]error, 0, limit)
	label := fmt.Sprintf("spec:%s", strings.TrimSpace(specName))

	for i := 0; i < limit; i++ {
		failure := failures[i]
		title := gateFailureTitle(failure)
		description := gateFailureDescription(failure)
		b, err := beadClient.Create(title, 0, []string{label}, []string{description})
		if err != nil {
			createErrors = append(createErrors, fmt.Errorf("create bead for %q: %w", failureName(failure), err))
			continue
		}
		if b == nil {
			createErrors = append(createErrors, fmt.Errorf("create bead for %q: returned nil bead", failureName(failure)))
			continue
		}
		ids = append(ids, b.ID)
	}

	if len(createErrors) > 0 {
		return ids, errors.Join(createErrors...)
	}
	return ids, nil
}

func gateFailureTitle(failure GateFailure) string {
	return fmt.Sprintf("Fix: %s", failureName(failure))
}

func gateFailureDescription(failure GateFailure) string {
	name := failureName(failure)
	message := strings.TrimSpace(failure.Message)
	suggestedFix := strings.TrimSpace(failure.SuggestedFix)

	parts := []string{
		fmt.Sprintf("Gate failure: %s", name),
		fmt.Sprintf("Message: %s", message),
	}
	if suggestedFix != "" {
		parts = append(parts, fmt.Sprintf("Suggested fix: %s", suggestedFix))
	}
	return strings.Join(parts, "\n")
}

func failureName(failure GateFailure) string {
	name := strings.TrimSpace(failure.TestName)
	if name == "" {
		return defaultGateFailureName
	}
	return name
}
