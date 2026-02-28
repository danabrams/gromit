package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/danabrams/gromit/internal/conversation"
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
	renderedPrompt, err := p.deps.ExploreRenderer.RenderExplore(exploreContext)
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

	// Resolve agent
	resolvedAgent, err := p.deps.AgentResolver.Resolve("explore", input.AgentName, input.ChooseAgent)
	if err != nil {
		return nil, fmt.Errorf("resolving agent: %w", err)
	}

	// Forward model to agent if model is specified and ModelForwarder is available
	if input.Model != "" && p.deps.ModelForwarder != nil {
		forwardedAgent, warning := p.deps.ModelForwarder(resolvedAgent, input.Model)
		if warning != "" && p.deps.WarningWriter != nil {
			p.deps.WarningWriter(warning)
		}
		resolvedAgent = forwardedAgent
	}

	// Launch agent
	if err := resolvedAgent.LaunchInDir(promptPath, ""); err != nil {
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
	existingSet := make(map[string]struct{}, len(existing))
	for _, idea := range existing {
		existingSet[idea.ID] = struct{}{}
	}

	newIDs := make([]string, 0, len(current))
	for _, idea := range current {
		if _, exists := existingSet[idea.ID]; !exists {
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
	return validateRequiredDeps([]namedDependency{
		{name: "AgentResolver", dep: p.deps.AgentResolver},
		{name: "ExploreRenderer", dep: p.deps.ExploreRenderer},
		{name: "BacklogClient", dep: p.deps.BacklogClient},
	})
}

// StreamSession wraps a stream-json-backed conversation session for explore.
type StreamSession struct {
	events    chan conversation.Event
	cancel    chan struct{}
	closeOnce sync.Once
}

// Events returns the channel of conversation events.
func (s *StreamSession) Events() <-chan conversation.Event {
	return s.events
}

// Cancel signals the stream to stop and closes the event channel.
func (s *StreamSession) Cancel() {
	s.closeOnce.Do(func() {
		close(s.cancel)
	})
}

// FollowUp is a no-op for explore sessions (not used for follow-up prompts).
func (s *StreamSession) FollowUp(prompt string) {
	// Explore sessions don't support follow-up prompts
}

// StartExploreSession launches a stream-json-backed explore session that emits
// ConversationEvents over a channel. The session is managed by the caller.
func (p *Pipeline) StartExploreSession(ctx context.Context, input ExploreInput) (conversation.Session, error) {
	if err := p.validateExploreDeps(); err != nil {
		return nil, err
	}

	// Create a session with event channel and cancel signal
	session := &StreamSession{
		events: make(chan conversation.Event, 10),
		cancel: make(chan struct{}),
	}

	// Start a goroutine to run the explore workflow and emit events
	go func() {
		defer close(session.events)

		// Build explore prompt
		exploreContext := &ExplorePromptInput{
			Query: input.Topic,
		}
		renderedPrompt, err := p.deps.ExploreRenderer.RenderExplore(exploreContext)
		if err != nil {
			select {
			case session.events <- conversation.Event{
				Type: conversation.EventTypeStream,
				Text: fmt.Sprintf("Error rendering explore prompt: %v", err),
			}:
			case <-session.cancel:
				return
			}
			return
		}

		// Write temp file
		tmpDir := filepath.Join(p.paths.GromitDir, "tmp")
		promptPath, cleanup, err := WriteTempPrompt(tmpDir, renderedPrompt)
		if err != nil {
			select {
			case session.events <- conversation.Event{
				Type: conversation.EventTypeStream,
				Text: fmt.Sprintf("Error writing temp prompt: %v", err),
			}:
			case <-session.cancel:
				return
			}
			return
		}
		defer cleanup()

		// Resolve agent
		agent, err := p.deps.AgentResolver.Resolve("explore", input.AgentName, input.ChooseAgent)
		if err != nil {
			select {
			case session.events <- conversation.Event{
				Type: conversation.EventTypeStream,
				Text: fmt.Sprintf("Error resolving agent: %v", err),
			}:
			case <-session.cancel:
				return
			}
			return
		}

		// Launch agent
		if err := agent.LaunchInDir(promptPath, ""); err != nil {
			select {
			case session.events <- conversation.Event{
				Type: conversation.EventTypeStream,
				Text: fmt.Sprintf("Error launching agent: %v", err),
			}:
			case <-session.cancel:
				return
			}
			return
		}

		// Emit completion event (unless cancelled)
		select {
		case session.events <- conversation.Event{
			Type: conversation.EventTypeDone,
		}:
		case <-session.cancel:
			return
		}
	}()

	return session, nil
}
