package runner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/specgate"
)

const (
	defaultGateFailureName = "unnamed gate failure"
	fixBeadPriority        = 0
	specLabelPrefix        = "spec:"
	fixBeadTitlePrefix     = "Fix: "
)

// GateFailure is a structured spec gate failure consumed by fix-bead synthesis.
type GateFailure struct {
	TestName     string `json:"test_name"`
	Message      string `json:"message"`
	SuggestedFix string `json:"suggested_fix"`
}

// SynthesizeFixBeads creates up to MaxSynthesizedSpecFixBeads P0 fix beads for gate failures.
func SynthesizeFixBeads(ctx context.Context, specName string, failures []GateFailure, beadClient BeadClient) ([]string, error) {
	if len(failures) == 0 {
		return []string{}, nil
	}
	if beadClient == nil {
		return nil, errors.New("bead client is required")
	}

	limit := len(failures)
	if limit > runtypes.MaxSynthesizedSpecFixBeads {
		limit = runtypes.MaxSynthesizedSpecFixBeads
		log.Printf(
			"runner: capped fix bead synthesis for spec %q at %d failures (%d remaining)",
			specName,
			runtypes.MaxSynthesizedSpecFixBeads,
			len(failures)-limit,
		)
	}

	ids := make([]string, 0, limit)
	createErrors := make([]error, 0, limit)
	label := specLabel(specName)

	for i := 0; i < limit; i++ {
		failure := failures[i]
		title := gateFailureTitle(failure)
		description := gateFailureDescription(failure)
		b, err := beadClient.Create(title, fixBeadPriority, []string{label}, []string{description})
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
	return fmt.Sprintf("%s%s", fixBeadTitlePrefix, failureName(failure))
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

func specLabel(specName string) string {
	return fmt.Sprintf("%s%s", specLabelPrefix, strings.TrimSpace(specName))
}

func convertFailedCriteria(failures []specgate.CriterionResult) []GateFailure {
	if len(failures) == 0 {
		return []GateFailure{}
	}

	converted := make([]GateFailure, 0, len(failures))
	for _, failure := range failures {
		converted = append(converted, GateFailure{
			TestName: failure.Criterion,
			Message:  failure.Evidence,
		})
	}
	return converted
}
