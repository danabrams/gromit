package pipeline

import (
	"context"
	"fmt"
)

// refineContext holds state for a refine workflow execution.
type refineContext struct {
	specsBeforeSnapshot []string
}

// Refine executes the refine workflow interactively.
func (p *Pipeline) Refine(ctx context.Context, input RefineInput) (*RefineSession, error) {
	if p.deps == nil || p.deps.AgentResolver == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	// Scan existing specs before launching agent
	existingSpecs, err := ListMarkdownFiles(p.paths.SpecsDir)
	if err != nil {
		return nil, fmt.Errorf("scanning existing specs: %w", err)
	}

	refineCtx := &refineContext{
		specsBeforeSnapshot: existingSpecs,
	}

	// Return session with context
	return &RefineSession{
		ctx: refineCtx,
	}, nil
}

// RefineSession is a typed wrapper for interactive Refine sessions.
type RefineSession struct {
	Session
	ctx *refineContext
}

// GetBeforeSnapshot returns the specs that existed before the session started (for testing).
func (s *RefineSession) GetBeforeSnapshot() []string {
	if s.ctx == nil {
		return nil
	}
	return s.ctx.specsBeforeSnapshot
}
