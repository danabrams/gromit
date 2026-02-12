package pipeline

import (
	"context"
	"fmt"
)

// Refine executes the refine workflow interactively.
func (p *Pipeline) Refine(ctx context.Context, input RefineInput) (*RefineSession, error) {
	if p.deps == nil || p.deps.AgentResolver == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	// Minimal implementation - return an empty session
	return &RefineSession{}, nil
}
