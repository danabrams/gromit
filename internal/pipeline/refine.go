package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
)

// Refine executes the refine workflow interactively.
func (p *Pipeline) Refine(ctx context.Context, input RefineInput) (*RefineResult, error) {
	if p.deps == nil || p.deps.AgentResolver == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	// Determine idea text
	ideaText := input.IdeaText
	ideaID := input.IdeaID

	// If IdeaID is provided, load from backlog
	if ideaID != "" {
		idea, err := p.deps.BacklogClient.Get(ideaID)
		if err != nil {
			return nil, fmt.Errorf("loading backlog idea: %w", err)
		}
		if idea == nil {
			return nil, fmt.Errorf("backlog idea not found: %s", ideaID)
		}

		// Combine text and context
		ideaText = idea.Text
		if idea.Context != "" {
			ideaText = fmt.Sprintf("%s\n\nContext: %s", idea.Text, idea.Context)
		}
	}

	// Scan existing specs before launching agent
	existingSpecs, err := ListMarkdownFiles(p.paths.SpecsDir)
	if err != nil {
		return nil, fmt.Errorf("scanning existing specs: %w", err)
	}

	// Build system prompt
	systemPrompt := p.buildRefinePrompt(ideaText)

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

	// Launch agent (blocks until complete)
	if err := agent.Launch(promptPath); err != nil {
		return nil, fmt.Errorf("launching agent: %w", err)
	}

	// Post-processing: detect new specs
	newSpecs, err := ListMarkdownFiles(p.paths.SpecsDir)
	if err != nil {
		return nil, fmt.Errorf("scanning specs after agent: %w", err)
	}

	createdSpecs := DiffFiles(existingSpecs, newSpecs)

	result := NewRefineResult()
	result.CreatedSpecs = createdSpecs

	return &result, nil
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
