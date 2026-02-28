package prepare

import (
	"context"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
)

// ReadinessOutcome captures the result of a structured readiness check.
type ReadinessOutcome string

const (
	// ReadinessOutcomeReady signals the bead is structurally ready.
	ReadinessOutcomeReady ReadinessOutcome = "ready"
	// ReadinessOutcomeNotReadyCriteria indicates missing or ambiguous criteria.
	ReadinessOutcomeNotReadyCriteria ReadinessOutcome = "not_ready_criteria"
	// ReadinessOutcomeNotReadyScope indicates scope problems with expected outputs.
	ReadinessOutcomeNotReadyScope ReadinessOutcome = "not_ready_scope"
)

const (
	ReasonCriteriaMissing   = "criteria_missing"
	ReasonCriteriaAmbiguous = "criteria_ambiguous"
	ReasonScopeTooBroad     = "scope_too_broad"
)

// ReadinessAssessor evaluates whether a bead satisfies structured readiness requirements.
type ReadinessAssessor interface {
	AssessStructured(ctx context.Context, b *bead.Bead) (ReadinessOutcome, string)
}

// CheckCriteriaPresence ensures at least one acceptance criterion exists for the bead.
func CheckCriteriaPresence(b *bead.Bead) (ReadinessOutcome, string) {
	if len(effectiveCriteria(b)) == 0 {
		return ReadinessOutcomeNotReadyCriteria, ReasonCriteriaMissing
	}
	return ReadinessOutcomeReady, ""
}

func effectiveCriteria(b *bead.Bead) []string {
	if b == nil {
		return nil
	}
	outputs := sanitizeOutputs(b.ExpectedOutputs)
	if len(outputs) > 0 {
		return outputs
	}
	return parseAcceptanceCriteria(b.AcceptanceCriteria)
}

func sanitizeOutputs(outputs []string) []string {
	cleaned := make([]string, 0, len(outputs))
	for _, output := range outputs {
		trimmed := strings.TrimSpace(output)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned
}

func parseAcceptanceCriteria(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		cleaned := strings.TrimSpace(line)
		if cleaned == "" {
			continue
		}
		trimmed = append(trimmed, cleaned)
	}
	return trimmed
}
