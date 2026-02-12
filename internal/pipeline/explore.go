package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
)

// Explore executes the explore workflow in non-interactive mode.
// It validates deps, records existing artifacts (epics, specs, backlog items) as pre-snapshots,
// builds explore prompt using renderer, writes temp file, resolves agent, launches agent,
// performs post-processing to detect new artifacts, and returns ExploreResult.
func (p *Pipeline) Explore(ctx context.Context, input ExploreInput) (*ExploreResult, error) {
	// Validate dependencies
	if err := p.validateExploreDeps(); err != nil {
		return nil, err
	}

	// Initialize result with empty slices
	result := NewExploreResult()

	// Record existing artifacts (pre-snapshots)
	var existingEpics, existingSpecs []string
	var err error

	if p.paths.EpicsDir != "" {
		existingEpics, err = ListMarkdownFiles(p.paths.EpicsDir)
		if err != nil {
			return nil, fmt.Errorf("scanning epics directory: %w", err)
		}
	}

	if p.paths.SpecsDir != "" {
		existingSpecs, err = ListMarkdownFiles(p.paths.SpecsDir)
		if err != nil {
			return nil, fmt.Errorf("scanning specs directory: %w", err)
		}
	}

	existingBacklogItems, err := p.deps.BacklogClient.List()
	if err != nil {
		return nil, fmt.Errorf("loading backlog: %w", err)
	}
	_ = existingBacklogItems // Will be used in post-processing
	_ = existingEpics        // Will be used in post-processing
	_ = existingSpecs        // Will be used in post-processing

	// Build explore prompt using renderer
	exploreContext := map[string]interface{}{
		"Topic": input.Topic,
	}
	renderedPrompt, err := p.deps.PromptRenderer.RenderExplore(exploreContext)
	if err != nil {
		return nil, fmt.Errorf("rendering explore prompt: %w", err)
	}

	// Write temp file
	tmpDir := filepath.Join(p.paths.GromitDir, "tmp")
	promptPath, cleanup, err := WriteTempPrompt(tmpDir, renderedPrompt)
	if err != nil {
		return nil, fmt.Errorf("writing temp prompt: %w", err)
	}
	defer cleanup()

	// Resolve agent (defaulting to "claude" if not specified)
	agentName := input.AgentName
	if agentName == "" {
		agentName = "claude"
	}
	agent, err := p.deps.AgentResolver.Resolve("explore", agentName, false)
	if err != nil {
		return nil, fmt.Errorf("resolving agent: %w", err)
	}

	// Launch agent
	if err := agent.Launch(promptPath); err != nil {
		return nil, fmt.Errorf("launching agent: %w", err)
	}

	// TODO: Post-processing: detect new artifacts using pre-snapshots

	return &result, nil
}

// validateExploreDeps checks that all required dependencies for Explore are present.
func (p *Pipeline) validateExploreDeps() error {
	if p.deps == nil {
		return fmt.Errorf("pipeline: nil dependencies")
	}

	requiredDeps := map[string]interface{}{
		"AgentResolver":  p.deps.AgentResolver,
		"PromptRenderer": p.deps.PromptRenderer,
		"BacklogClient":  p.deps.BacklogClient,
	}

	for name, dep := range requiredDeps {
		if dep == nil {
			return fmt.Errorf("pipeline: nil %s", name)
		}
	}

	return nil
}
