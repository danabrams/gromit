package prompt

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/readiness"
)

// PromptReadinessAssessor determines whether a bead is ready from a prompt perspective.
// It implements the readiness.Assessor interface to integrate with the Gate stage.
type PromptReadinessAssessor struct {
}

// NewPromptReadinessAssessor creates a new PromptReadinessAssessor.
func NewPromptReadinessAssessor() *PromptReadinessAssessor {
	return &PromptReadinessAssessor{}
}

// Assess determines whether a bead should be blocked from proceeding based on
// prompt readiness criteria. Currently returns StatusReady for all beads.
func (p *PromptReadinessAssessor) Assess(ctx context.Context, b *bead.Bead) (readiness.Assessment, error) {
	// Stub implementation: no beads are blocked for readiness reasons yet
	return readiness.Assessment{Status: readiness.StatusReady}, nil
}
