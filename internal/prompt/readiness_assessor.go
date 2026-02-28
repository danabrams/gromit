package prompt

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
)

// PromptReadinessAssessor determines whether a bead is ready from a prompt perspective.
// It implements the DataQualityBlocker interface to integrate with the Gate stage.
type PromptReadinessAssessor struct {
}

// NewPromptReadinessAssessor creates a new PromptReadinessAssessor.
func NewPromptReadinessAssessor() *PromptReadinessAssessor {
	return &PromptReadinessAssessor{}
}

// ShouldBlock determines whether a bead should be blocked from proceeding based on
// prompt readiness criteria. Currently returns false (not blocked) for all beads.
// Returns (blocked, reason, error).
func (p *PromptReadinessAssessor) ShouldBlock(ctx context.Context, b *bead.Bead) (bool, string, error) {
	// Stub implementation: no beads are blocked for readiness reasons yet
	return false, "", nil
}
