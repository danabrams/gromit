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
	existingEpics, err := scanArtifacts(p.paths.EpicsDir, "epics directory")
	if err != nil {
		return nil, err
	}

	existingSpecs, err := scanArtifacts(p.paths.SpecsDir, "specs directory")
	if err != nil {
		return nil, err
	}

	existingBacklogItems, err := p.deps.BacklogClient.List()
	if err != nil {
		return nil, fmt.Errorf("loading backlog: %w", err)
	}

	// Build explore prompt using renderer
	exploreContext := &ExplorePromptInput{
		Query: input.Topic,
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

	// Post-processing: detect new artifacts using pre-snapshots
	currentEpics, err := scanArtifacts(p.paths.EpicsDir, "epics directory after session")
	if err != nil {
		return nil, err
	}

	currentSpecs, err := scanArtifacts(p.paths.SpecsDir, "specs directory after session")
	if err != nil {
		return nil, err
	}

	currentBacklogItems, err := p.deps.BacklogClient.List()
	if err != nil {
		return nil, fmt.Errorf("loading backlog after session: %w", err)
	}

	// Diff to find new artifacts and ensure non-nil slices
	result.CreatedEpics = ensureNonNil(DiffFiles(existingEpics, currentEpics))
	result.CreatedSpecs = ensureNonNil(DiffFiles(existingSpecs, currentSpecs))
	result.CreatedBacklogItems = ensureNonNil(diffBacklogItems(existingBacklogItems, currentBacklogItems))

	return &result, nil
}

// scanArtifacts scans a directory for markdown files, returning empty slice if dir is empty.
// contextName is used in error messages (e.g., "epics directory", "specs directory after session").
func scanArtifacts(dir, contextName string) ([]string, error) {
	if dir == "" {
		return []string{}, nil
	}
	files, err := ListMarkdownFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", contextName, err)
	}
	return files, nil
}

// ensureNonNil returns the input slice or an empty slice if input is nil.
// This ensures consistent nil-safe slice handling across all result fields.
func ensureNonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// diffBacklogItems returns IDs of backlog items in current that are not in existing.
func diffBacklogItems(existing, current []*Idea) []string {
	existingSet := make(map[string]bool)
	for _, idea := range existing {
		existingSet[idea.ID] = true
	}

	var newIDs []string
	for _, idea := range current {
		if !existingSet[idea.ID] {
			newIDs = append(newIDs, idea.ID)
		}
	}

	return newIDs
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
