package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
