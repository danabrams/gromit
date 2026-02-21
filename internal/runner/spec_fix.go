package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/specgate"
)

const (
	defaultGateFailureName = "unnamed gate failure"
	fixBeadPriority        = 0
	fixBeadPriorityLabel   = "P0"
)

// GateFailure is a structured spec gate failure consumed by fix-bead synthesis.
type GateFailure struct {
	TestName     string `json:"test_name"`
	Message      string `json:"message"`
	SuggestedFix string `json:"suggested_fix"`
}

// SynthesizeFixBeads creates up to runtypes.MaxSynthesizedSpecFixBeads P0 fix beads for gate failures.
func SynthesizeFixBeads(ctx context.Context, specName string, failures []GateFailure, beadClient BeadClient) ([]string, error) {
	if len(failures) == 0 {
		return []string{}, nil
	}
	if beadClient == nil {
		return nil, errors.New("bead client is required")
	}

	return specgate.SynthesizeFixBeads(ctx, specName, toCriterionResults(failures), fixBeadPriorityLabel, &runnerBeadCreator{
		beadClient: beadClient,
	})
}

func failureName(failure GateFailure) string {
	name := strings.TrimSpace(failure.TestName)
	if name == "" {
		return defaultGateFailureName
	}
	return name
}

type runnerBeadCreator struct {
	beadClient BeadClient
}

var _ specgate.BeadCreator = (*runnerBeadCreator)(nil)

func (r *runnerBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	_ = ctx
	_ = priority

	b, err := r.beadClient.Create(title, fixBeadPriority, labels, []string{description})
	if err != nil {
		return "", err
	}
	if b == nil {
		return "", errors.New("returned nil bead")
	}
	return b.ID, nil
}

func toCriterionResults(failures []GateFailure) []specgate.CriterionResult {
	if len(failures) == 0 {
		return []specgate.CriterionResult{}
	}

	results := make([]specgate.CriterionResult, 0, len(failures))
	for _, failure := range failures {
		results = append(results, specgate.CriterionResult{
			Criterion: failureName(failure),
			Evidence:  failureEvidence(failure),
		})
	}
	return results
}

func failureEvidence(failure GateFailure) string {
	message := strings.TrimSpace(failure.Message)
	suggestedFix := strings.TrimSpace(failure.SuggestedFix)
	if suggestedFix == "" {
		return message
	}
	if message == "" {
		return fmt.Sprintf("Suggested fix: %s", suggestedFix)
	}
	return fmt.Sprintf("%s\nSuggested fix: %s", message, suggestedFix)
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
