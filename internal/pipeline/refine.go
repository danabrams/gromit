package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
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

	// Build system prompt
	systemPrompt := p.buildRefinePrompt(input.IdeaText)

	// Write prompt to temp file
	tmpDir := filepath.Join(p.paths.GromitDir, "tmp")
	promptPath, cleanup, err := WriteTempPrompt(tmpDir, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("writing temp prompt: %w", err)
	}
	defer cleanup()

	// Resolve agent
	agent, err := p.deps.AgentResolver.Resolve("refine", input.AgentName, false)
	if err != nil {
		return nil, fmt.Errorf("resolving agent: %w", err)
	}

	// Launch agent
	if err := agent.Launch(promptPath); err != nil {
		return nil, fmt.Errorf("launching agent: %w", err)
	}

	refineCtx := &refineContext{
		specsBeforeSnapshot: existingSpecs,
	}

	// Return session with context
	return &RefineSession{
		ctx: refineCtx,
	}, nil
}

// buildRefinePrompt constructs the system prompt for refine sessions.
func (p *Pipeline) buildRefinePrompt(ideaText string) string {
	if ideaText == "" {
		// Blank session
		return fmt.Sprintf(`## Context

Specs directory: %s

## Instructions

(skill content would go here)`, p.paths.SpecsDir)
	}

	// Normal session with idea text
	return fmt.Sprintf(`Idea to refine:

%s

## Context

Specs directory: %s

## Instructions

(skill content would go here)`, ideaText, p.paths.SpecsDir)
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
