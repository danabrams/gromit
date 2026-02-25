package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestPipelineRefine_ChooseAgentWiring ensures the choose-agent flag is parsed
// into RefineInput and propagated to AgentResolver.Resolve.
func TestPipelineRefine_ChooseAgentWiring(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	var capturedChoose bool
	agent := &mockAgent{
		LaunchInDirFn: func(promptPath, dir string) error {
			return nil
		},
	}
	resolver := &mockAgentResolver{
		ResolveFn: func(phase, flagOverride string, choosePicker bool) (Agent, error) {
			if phase != "refine" {
				t.Fatalf("unexpected phase %q", phase)
			}
			capturedChoose = choosePicker
			return agent, nil
		},
	}

	deps := &Deps{AgentResolver: resolver}
	paths := &Paths{GromitDir: gromitDir, SpecsDir: specsDir}
	p := New(deps, paths)

	input := RefineInput{IdeaText: "test idea"}
	inputValue := reflect.ValueOf(&input).Elem()
	chooseField := inputValue.FieldByName("ChooseAgent")
	if !chooseField.IsValid() {
		t.Fatalf("RefineInput is missing ChooseAgent field")
	}
	if chooseField.Kind() != reflect.Bool {
		t.Fatalf("RefineInput.ChooseAgent is not bool")
	}
	if !chooseField.CanSet() {
		t.Fatalf("RefineInput.ChooseAgent cannot be set")
	}
	chooseField.SetBool(true)

	if _, err := p.Refine(context.Background(), input); err != nil {
		t.Fatalf("Refine() failed: %v", err)
	}

	if !capturedChoose {
		t.Errorf("AgentResolver.Resolve choosePicker = %v, want true when ChooseAgent is set", capturedChoose)
	}
}
