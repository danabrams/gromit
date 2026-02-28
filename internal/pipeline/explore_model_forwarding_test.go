package pipeline

import (
	"context"
	"path/filepath"
	"testing"
)

// TestExplore_CallsModelForwarderWhenModelNonEmpty verifies that Explore calls
// the model forwarder when ExploreInput.Model is non-empty.
func TestExplore_CallsModelForwarderWhenModelNonEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	var modelForwarderCalled bool
	var forwardedModel string
	var forwardedAgentName string

	mockAgent := &mockAgent{
		NameFn: func() string {
			return "codex"
		},
		LaunchInDirFn: func(promptPath, dir string) error {
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver:   mockAgentResolver,
		ExploreRenderer: &testExploreRenderer{},
		BacklogClient:   &testBacklogClient{},
		ModelForwarder: func(agent Agent, model string) (Agent, string) {
			modelForwarderCalled = true
			forwardedModel = model
			forwardedAgentName = agent.Name()
			return agent, ""
		},
	}

	paths := &Paths{
		GromitDir: gromitDir,
	}

	p := New(deps, paths)
	ctx := context.Background()

	input := ExploreInput{
		Topic: "test topic",
		Model: "gpt-4-codex",
	}

	_, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	if !modelForwarderCalled {
		t.Error("ModelForwarder should be called when Model is non-empty")
	}

	if forwardedModel != "gpt-4-codex" {
		t.Errorf("ModelForwarder called with model=%q, want %q", forwardedModel, "gpt-4-codex")
	}

	if forwardedAgentName != "codex" {
		t.Errorf("ModelForwarder called with agent=%q, want %q", forwardedAgentName, "codex")
	}
}

// TestExplore_SkipsModelForwarderWhenModelEmpty verifies that when ExploreInput.Model
// is empty, ModelForwarder is not called.
func TestExplore_SkipsModelForwarderWhenModelEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	var modelForwarderCalled bool

	mockAgent := &mockAgent{
		NameFn: func() string {
			return "codex"
		},
		LaunchInDirFn: func(promptPath, dir string) error {
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver:   mockAgentResolver,
		ExploreRenderer: &testExploreRenderer{},
		BacklogClient:   &testBacklogClient{},
		ModelForwarder: func(agent Agent, model string) (Agent, string) {
			modelForwarderCalled = true
			return agent, ""
		},
	}

	paths := &Paths{
		GromitDir: gromitDir,
	}

	p := New(deps, paths)
	ctx := context.Background()

	// Model is NOT set (empty string)
	input := ExploreInput{
		Topic: "test topic",
		Model: "",
	}

	_, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	if modelForwarderCalled {
		t.Error("ModelForwarder should not be called when Model is empty")
	}
}

// TestExplore_WritesWarningWhenModelForwardingUnsupported verifies that when
// ExploreInput.Model is non-empty but model forwarding is unsupported,
// Explore writes a warning via the WarningWriter and continues with original agent.
func TestExplore_WritesWarningWhenModelForwardingUnsupported(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")

	var capturedWarning string

	mockAgent := &mockAgent{
		NameFn: func() string {
			return "claude"
		},
		LaunchInDirFn: func(promptPath, dir string) error {
			return nil
		},
	}

	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			return mockAgent, nil
		},
	}

	deps := &Deps{
		AgentResolver:   mockAgentResolver,
		ExploreRenderer: &testExploreRenderer{},
		BacklogClient:   &testBacklogClient{},
		ModelForwarder: func(agent Agent, model string) (Agent, string) {
			// Simulate unsupported agent by returning a warning
			return agent, "model forwarding not supported for agent " + agent.Name()
		},
		WarningWriter: func(warning string) {
			capturedWarning = warning
		},
	}

	paths := &Paths{
		GromitDir: gromitDir,
	}

	p := New(deps, paths)
	ctx := context.Background()

	input := ExploreInput{
		Topic: "test topic",
		Model: "some-model",
	}

	result, err := p.Explore(ctx, input)
	if err != nil {
		t.Fatalf("Explore() should continue despite unsupported model forwarding, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Explore() returned nil result, but should continue despite warning")
	}

	if capturedWarning == "" {
		t.Error("WarningWriter should be called when model forwarding is unsupported")
	}

	if capturedWarning != "" && capturedWarning != "model forwarding not supported for agent claude" {
		t.Errorf("Warning = %q, want %q", capturedWarning, "model forwarding not supported for agent claude")
	}
}
