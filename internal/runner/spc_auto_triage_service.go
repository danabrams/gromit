package runner

import (
	"context"
)

// SPCAutoTriageService evaluates SPC signals after a run and wires automated triage logic.
type SPCAutoTriageService interface {
	// EvaluateAndTriage runs the auto-triage workflow and returns any diagnostic error encountered.
	EvaluateAndTriage(ctx context.Context) error
}

type noopSPCAutoTriageService struct{}

func newSPCAutoTriageService() SPCAutoTriageService {
	return &noopSPCAutoTriageService{}
}

func (noopSPCAutoTriageService) EvaluateAndTriage(ctx context.Context) error {
	return nil
}
