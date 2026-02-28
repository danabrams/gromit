package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/conversation"
)

// TestExploreInput_ChooseAgentFieldExists verifies ExploreInput includes ChooseAgent for picker wiring.
func TestExploreInput_ChooseAgentFieldExists(t *testing.T) {
	// Expected failure: ExploreInput.ChooseAgent field does not exist yet
	inputType := reflect.TypeOf(ExploreInput{})
	if _, ok := inputType.FieldByName("ChooseAgent"); !ok {
		t.Fatalf("ExploreInput is missing ChooseAgent field for agent picker wiring")
	}
}

// TestPipelineExplore_PropagatesChooseAgentToResolver verifies choose-agent flag reaches AgentResolver.
func TestPipelineExplore_PropagatesChooseAgentToResolver(t *testing.T) {
	// Expected failure: Pipeline.Explore does not pass ExploreInput.ChooseAgent to AgentResolver.Resolve yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}

	var capturedChoose bool
	mockAgent := &mockAgent{
		LaunchFn: func(promptPath string) error {
			return nil
		},
	}
	mockAgentResolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			capturedChoose = choosePicker
			return mockAgent, nil
		},
	}
	mockRenderer := &mockExploreRenderer{
		RenderExploreFn: func(input *ExplorePromptInput) (string, error) {
			return "explore prompt", nil
		},
	}
	mockBacklog := &mockBacklogClient{
		ListFn: func() ([]*Idea, error) {
			return []*Idea{}, nil
		},
	}

	deps := &Deps{
		AgentResolver:   mockAgentResolver,
		ExploreRenderer: mockRenderer,
		BacklogClient:   mockBacklog,
	}
	paths := &Paths{
		GromitDir: gromitDir,
	}

	input := ExploreInput{
		Topic:     "test topic",
		AgentName: "test-agent",
		Model:     "sonnet",
	}

	inputValue := reflect.ValueOf(&input).Elem()
	chooseField := inputValue.FieldByName("ChooseAgent")
	if !chooseField.IsValid() {
		t.Fatalf("ExploreInput missing ChooseAgent field (expected for choose-agent wiring)")
	}
	if chooseField.Kind() == reflect.Bool && chooseField.CanSet() {
		chooseField.SetBool(true)
	}

	p := New(deps, paths)
	if _, err := p.Explore(context.Background(), input); err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	if !capturedChoose {
		t.Errorf("AgentResolver.Resolve choosePicker = false, want true when ChooseAgent is set")
	}
}

func TestPipelineExplore_ModelForwardingCases(t *testing.T) {
	type scenario struct {
		name                  string
		agentName             string
		model                 string
		expectModelForward    bool
		expectForwardedLaunch bool
		expectOriginalLaunch  bool
		warningMsg            string
	}

	scenarios := []scenario{
		{
			name:                  "codex propagation",
			agentName:             "codex",
			model:                 "gpt-5.3-codex",
			expectModelForward:    true,
			expectForwardedLaunch: true,
			expectOriginalLaunch:  false,
		},
		{
			name:                  "gemini propagation",
			agentName:             "gemini",
			model:                 "gemini-2.5-pro",
			expectModelForward:    true,
			expectForwardedLaunch: true,
			expectOriginalLaunch:  false,
		},
		{
			name:                  "unsupported-agent warning and continue",
			agentName:             "claude",
			model:                 "sonnet",
			expectModelForward:    true,
			expectForwardedLaunch: false,
			expectOriginalLaunch:  true,
			warningMsg:            "model forwarding not supported for agent claude",
		},
		{
			name:                 "empty model no-op",
			agentName:            "codex",
			model:                "",
			expectModelForward:   false,
			expectOriginalLaunch: true,
		},
	}

	for _, tc := range scenarios {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gromitDir := t.TempDir()
			specsDir := filepath.Join(gromitDir, "specs")
			epicsDir := filepath.Join(gromitDir, "epics")
			if err := os.MkdirAll(specsDir, 0o755); err != nil {
				t.Fatalf("failed to create specs dir: %v", err)
			}
			if err := os.MkdirAll(epicsDir, 0o755); err != nil {
				t.Fatalf("failed to create epics dir: %v", err)
			}

			forwardedAgentLaunched := false
			originalAgentLaunched := false
			modelForwardCalled := false
			warningCaptured := ""

			forwardedAgent := &mockAgent{
				NameFn: func() string { return tc.agentName },
				LaunchInDirFn: func(promptPath, dir string) error {
					forwardedAgentLaunched = true
					return nil
				},
			}
			originalAgent := &mockAgent{
				NameFn: func() string { return tc.agentName },
				LaunchInDirFn: func(promptPath, dir string) error {
					originalAgentLaunched = true
					return nil
				},
			}

			deps := &Deps{
				AgentResolver: &mockAgentResolver{
					ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
						return originalAgent, nil
					},
				},
				ExploreRenderer: &mockExploreRenderer{
					RenderExploreFn: func(input *ExplorePromptInput) (string, error) {
						return "prompt", nil
					},
				},
				BacklogClient: &mockBacklogClient{
					ListFn: func() ([]*Idea, error) {
						return []*Idea{}, nil
					},
				},
				ModelForwarder: func(agent Agent, model string) (Agent, string) {
					modelForwardCalled = true
					if model != tc.model {
						t.Fatalf("model = %q, want %q", model, tc.model)
					}
					if tc.warningMsg != "" {
						return agent, tc.warningMsg
					}
					return forwardedAgent, ""
				},
				WarningWriter: func(message string) {
					warningCaptured = message
				},
			}

			paths := &Paths{
				GromitDir: gromitDir,
				SpecsDir:  specsDir,
				EpicsDir:  epicsDir,
			}

			p := New(deps, paths)
			if _, err := p.Explore(context.Background(), ExploreInput{Topic: "topic", Model: tc.model}); err != nil {
				t.Fatalf("Explore() failed: %v", err)
			}

			if tc.expectModelForward && !modelForwardCalled {
				t.Fatalf("ModelForwarder was not called")
			}
			if !tc.expectModelForward && modelForwardCalled {
				t.Fatalf("ModelForwarder was unexpectedly called")
			}

			if forwardedAgentLaunched != tc.expectForwardedLaunch {
				t.Fatalf("forwarded launch = %v, want %v", forwardedAgentLaunched, tc.expectForwardedLaunch)
			}
			if originalAgentLaunched != tc.expectOriginalLaunch {
				t.Fatalf("original launch = %v, want %v", originalAgentLaunched, tc.expectOriginalLaunch)
			}
			if warningCaptured != tc.warningMsg {
				t.Fatalf("warning captured = %q, want %q", warningCaptured, tc.warningMsg)
			}
		})
	}
}

// TestExplore_ModelReachesAgentLaunch verifies that ExploreInput.Model is propagated
// through ModelForwarder to the actual agent.LaunchInDir call (regression test).
func TestExplore_ModelReachesAgentLaunch(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	epicsDir := filepath.Join(gromitDir, "epics")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	if err := os.MkdirAll(epicsDir, 0o755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Track which agent's LaunchInDir was called
	originalAgentLaunched := false
	forwardedAgentLaunched := false

	originalAgent := &mockAgent{
		NameFn: func() string { return "claude" },
		LaunchInDirFn: func(promptPath, dir string) error {
			originalAgentLaunched = true
			return nil
		},
	}

	forwardedAgent := &mockAgent{
		NameFn: func() string { return "claude-with-model" },
		LaunchInDirFn: func(promptPath, dir string) error {
			forwardedAgentLaunched = true
			return nil
		},
	}

	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return originalAgent, nil
			},
		},
		ExploreRenderer: &mockExploreRenderer{
			RenderExploreFn: func(input *ExplorePromptInput) (string, error) {
				return "explore prompt", nil
			},
		},
		BacklogClient: &mockBacklogClient{
			ListFn: func() ([]*Idea, error) {
				return []*Idea{}, nil
			},
		},
		// ModelForwarder returns a different agent to prove model is propagated
		ModelForwarder: func(agent Agent, model string) (Agent, string) {
			if model == "sonnet" {
				return forwardedAgent, ""
			}
			return agent, ""
		},
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}

	p := New(deps, paths)
	input := ExploreInput{
		Topic: "test topic",
		Model: "sonnet", // Non-empty model should reach the agent launch
	}

	if _, err := p.Explore(context.Background(), input); err != nil {
		t.Fatalf("Explore() failed: %v", err)
	}

	// Verify that the forwarded agent (with model) was launched, not the original
	if !forwardedAgentLaunched {
		t.Fatalf("forwardedAgent.LaunchInDir was not called; model did not reach agent launch")
	}
	if originalAgentLaunched {
		t.Fatalf("originalAgent.LaunchInDir was called; model was not properly forwarded")
	}
}

// TestStartExploreSession_ModelReachesAgentLaunch verifies that ExploreInput.Model is propagated
// through ModelForwarder to the actual agent.LaunchInDir call in StartExploreSession (regression test).
func TestStartExploreSession_ModelReachesAgentLaunch(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	epicsDir := filepath.Join(gromitDir, "epics")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}
	if err := os.MkdirAll(epicsDir, 0o755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Track which agent's LaunchInDir was called
	originalAgentLaunched := false
	forwardedAgentLaunched := false

	originalAgent := &mockAgent{
		NameFn: func() string { return "claude" },
		LaunchInDirFn: func(promptPath, dir string) error {
			originalAgentLaunched = true
			return nil
		},
	}

	forwardedAgent := &mockAgent{
		NameFn: func() string { return "claude-with-model" },
		LaunchInDirFn: func(promptPath, dir string) error {
			forwardedAgentLaunched = true
			return nil
		},
	}

	deps := &Deps{
		AgentResolver: &mockAgentResolver{
			ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
				return originalAgent, nil
			},
		},
		ExploreRenderer: &mockExploreRenderer{
			RenderExploreFn: func(input *ExplorePromptInput) (string, error) {
				return "explore prompt", nil
			},
		},
		BacklogClient: &mockBacklogClient{
			ListFn: func() ([]*Idea, error) {
				return []*Idea{}, nil
			},
		},
		// ModelForwarder returns a different agent to prove model is propagated
		ModelForwarder: func(agent Agent, model string) (Agent, string) {
			if model == "sonnet" {
				return forwardedAgent, ""
			}
			return agent, ""
		},
	}

	paths := &Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}

	p := New(deps, paths)
	input := ExploreInput{
		Topic: "test topic",
		Model: "sonnet", // Non-empty model should reach the agent launch
	}

	session, err := p.StartExploreSession(context.Background(), input)
	if err != nil {
		t.Fatalf("StartExploreSession() failed: %v", err)
	}

	// Consume all events until completion
	for event := range session.Events() {
		if event.Type == conversation.EventTypeDone {
			break
		}
	}

	// Verify that the forwarded agent (with model) was launched, not the original
	if !forwardedAgentLaunched {
		t.Fatalf("forwardedAgent.LaunchInDir was not called; model did not reach agent launch in StartExploreSession")
	}
	if originalAgentLaunched {
		t.Fatalf("originalAgent.LaunchInDir was called; model was not properly forwarded in StartExploreSession")
	}
}
