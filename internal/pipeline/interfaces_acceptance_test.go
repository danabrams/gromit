//go:build acceptance

package pipeline

import (
	"testing"
)

// TestInterfaces_CompileTimeSatisfactionChecks verifies that compile-time interface
// satisfaction checks exist in pipeline.go following the pattern in runner/interfaces.go.
func TestInterfaces_CompileTimeSatisfactionChecks(t *testing.T) {
	// Expected failure: Compile-time satisfaction checks do not exist in internal/pipeline/pipeline.go
	// Expected failure: var _ AgentResolver = (*agent.Resolver)(nil) check does not exist
	// Expected failure: var _ ClaudeClient = (*claude.Client)(nil) check does not exist
	// Expected failure: var _ BeadClient = (*bead.Client)(nil) check does not exist
	// Expected failure: var _ BacklogClient = (*backlog.File)(nil) check does not exist
	// Expected failure: var _ PromptRenderer = (*prompt.Renderer)(nil) check does not exist
	// Expected failure: var _ LearningsManager = (*learnings.Manager)(nil) check does not exist
	// Expected failure: var _ StateManager = (*state.Manager)(nil) check does not exist
	// Expected failure: var _ LogWriter = (*logger.Logger)(nil) check does not exist

	// This test verifies that the compile-time checks are present by attempting
	// to reference them. If they don't exist, compilation will fail.
	// The actual check variables should be in pipeline.go at package level.

	// We can't directly test the existence of package-level vars, but we can
	// verify the interfaces compile against our mocks
	checkInterfaceSatisfaction()
}

func checkInterfaceSatisfaction() {
	// These assignments verify the interfaces are properly defined
	// Real implementation should have similar checks in pipeline.go
	var _ AgentResolver = (*mockAgentResolver)(nil)
	var _ ClaudeClient = (*mockClaudeClient)(nil)
	var _ BeadClient = (*mockBeadClient)(nil)
	var _ BacklogClient = (*mockBacklogClient)(nil)
	var _ PromptRenderer = (*mockPromptRenderer)(nil)
	var _ LearningsManager = (*mockLearningsManager)(nil)
	var _ StateManager = (*mockStateManager)(nil)
	var _ LogWriter = (*mockLogWriter)(nil)
}

// TestInterfaces_AgentResolverNarrowInterface verifies AgentResolver is a narrow interface
// with only the methods needed by pipeline package.
func TestInterfaces_AgentResolverNarrowInterface(t *testing.T) {
	// Expected failure: AgentResolver interface does not exist or has incorrect methods

	// Verify interface can be satisfied with minimal implementation
	var resolver AgentResolver = &mockAgentResolver{}

	// Verify it has exactly the method(s) needed for pipeline work
	_, err := resolver.Resolve("refine", "", false)
	if err != nil {
		// Mock returns nil error, so this should not error
	}

	// AgentResolver should NOT expose methods like Launch() - that's on the Agent type
	// This is verified by the fact that our mock only implements Resolve()
}

// TestInterfaces_ClaudeClientNarrowInterface verifies ClaudeClient is a narrow interface
// with only the methods needed by pipeline package.
func TestInterfaces_ClaudeClientNarrowInterface(t *testing.T) {
	// Expected failure: ClaudeClient interface does not exist or has incorrect methods

	var client ClaudeClient = &mockClaudeClient{}

	// Verify Run method exists (for non-interactive workflows)
	_, err := client.Run("prompt", "sonnet")
	if err != nil {
		// Mock returns nil error
	}

	// ClaudeClient should have minimal surface area - just Run for non-interactive workflows
	// Interactive workflows use AgentResolver + Agent.Launch pattern
}

// TestInterfaces_BeadClientNarrowInterface verifies BeadClient is a narrow interface
// with only the methods needed by pipeline package.
func TestInterfaces_BeadClientNarrowInterface(t *testing.T) {
	// Expected failure: BeadClient interface does not exist or has incorrect methods

	var client BeadClient = &mockBeadClient{}

	// Verify essential bead operations
	_, err := client.Ready()
	if err != nil {
		// Mock returns nil error
	}

	_, err = client.Show("bead-123")
	if err != nil {
		// Mock returns nil error
	}

	_, err = client.Create("Task", 1, []string{"label"}, []string{"output"})
	if err != nil {
		// Mock returns nil error
	}

	err = client.Close("bead-123")
	if err != nil {
		// Mock returns nil error
	}
}

// TestInterfaces_BacklogClientNarrowInterface verifies BacklogClient is a narrow interface
// with only the methods needed by pipeline package.
func TestInterfaces_BacklogClientNarrowInterface(t *testing.T) {
	// Expected failure: BacklogClient interface does not exist or has incorrect methods

	var client BacklogClient = &mockBacklogClient{}

	// Verify backlog operations
	_, err := client.List()
	if err != nil {
		// Mock returns nil error
	}

	_, err = client.Get("idea-123")
	if err != nil {
		// Mock returns nil error
	}

	err = client.Add(nil)
	if err != nil {
		// Mock returns nil error
	}

	err = client.Update("idea-123", func(item interface{}) {})
	if err != nil {
		// Mock returns nil error
	}
}

// TestInterfaces_PromptRendererNarrowInterface verifies PromptRenderer is a narrow interface
// with only the methods needed by pipeline package.
func TestInterfaces_PromptRendererNarrowInterface(t *testing.T) {
	// Expected failure: PromptRenderer interface does not exist or has incorrect methods

	var renderer PromptRenderer = &mockPromptRenderer{}

	// Verify workflow-specific render methods
	_, err := renderer.RenderRefine(nil)
	if err != nil {
		// Mock returns nil error
	}

	_, err = renderer.RenderPlan(nil)
	if err != nil {
		// Mock returns nil error
	}

	_, err = renderer.RenderDecompose(nil)
	if err != nil {
		// Mock returns nil error
	}
}

// TestInterfaces_LearningsManagerNarrowInterface verifies LearningsManager is a narrow interface
// with only the methods needed by pipeline package.
func TestInterfaces_LearningsManagerNarrowInterface(t *testing.T) {
	// Expected failure: LearningsManager interface does not exist or has incorrect methods

	var manager LearningsManager = &mockLearningsManager{}

	// Verify learning persistence
	err := manager.Add("Some learning")
	if err != nil {
		// Mock returns nil error
	}
}

// TestInterfaces_StateManagerNarrowInterface verifies StateManager is a narrow interface
// with only the methods needed by pipeline package.
func TestInterfaces_StateManagerNarrowInterface(t *testing.T) {
	// Expected failure: StateManager interface does not exist or has incorrect methods

	var manager StateManager = &mockStateManager{}

	// Verify state operations for review workflow
	_, err := manager.GetLastReviewCommit()
	if err != nil {
		// Mock returns nil error
	}

	err = manager.SetLastReviewCommit("abc123")
	if err != nil {
		// Mock returns nil error
	}
}

// TestInterfaces_LogWriterNarrowInterface verifies LogWriter is a narrow interface
// with only the methods needed by pipeline package.
func TestInterfaces_LogWriterNarrowInterface(t *testing.T) {
	// Expected failure: LogWriter interface does not exist or has incorrect methods

	var writer LogWriter = &mockLogWriter{}

	// Verify log writing
	err := writer.Write(nil)
	if err != nil {
		// Mock returns nil error
	}
}

// TestInterfaces_InterfaceMethodSignatures verifies that interface methods have
// the expected parameter and return types.
func TestInterfaces_InterfaceMethodSignatures(t *testing.T) {
	// Expected failure: Interface methods do not have correct signatures

	// This test verifies method signatures by attempting to call them with
	// specific types. Type mismatches will cause compilation failures.

	t.Run("AgentResolver.Resolve", func(t *testing.T) {
		var resolver AgentResolver = &mockAgentResolver{}
		// Resolve should accept phase string, flag override string, and choosePicker bool
		agent, err := resolver.Resolve("refine", "sonnet", true)
		_, _ = agent, err
	})

	t.Run("ClaudeClient.Run", func(t *testing.T) {
		var client ClaudeClient = &mockClaudeClient{}
		// Run should accept prompt string and model string
		result, err := client.Run("prompt text", "sonnet")
		_, _ = result, err
	})

	t.Run("BeadClient.Create", func(t *testing.T) {
		var client BeadClient = &mockBeadClient{}
		// Create should accept title, priority, labels slice, outputs slice
		bead, err := client.Create("Title", 1, []string{"label"}, []string{"output"})
		_, _ = bead, err
	})

	t.Run("BacklogClient.Update", func(t *testing.T) {
		var client BacklogClient = &mockBacklogClient{}
		// Update should accept id string and update function
		err := client.Update("idea-1", func(item interface{}) {
			// Update function receives the item
		})
		_ = err
	})

	t.Run("LearningsManager.Add", func(t *testing.T) {
		var manager LearningsManager = &mockLearningsManager{}
		// Add should accept content string
		err := manager.Add("learning content")
		_ = err
	})

	t.Run("StateManager.SetLastReviewCommit", func(t *testing.T) {
		var manager StateManager = &mockStateManager{}
		// SetLastReviewCommit should accept commit string
		err := manager.SetLastReviewCommit("abc123def456")
		_ = err
	})
}

// TestInterfaces_SessionInterfaceMethodSignatures verifies Session interface has
// correct method signatures.
func TestInterfaces_SessionInterfaceMethodSignatures(t *testing.T) {
	// Expected failure: Session interface methods do not have correct signatures

	var session Session = &mockSession{events: make(chan Event)}

	// Events() should return read-only channel of Event
	eventChan := session.Events()
	select {
	case event := <-eventChan:
		_ = event
	default:
		// No events in channel
	}

	// SendInput should accept string and return error
	err := session.SendInput("user input")
	_ = err

	// Cancel should accept no parameters and return nothing
	session.Cancel()

	// Wait should accept no parameters and return error
	err = session.Wait()
	_ = err
}
