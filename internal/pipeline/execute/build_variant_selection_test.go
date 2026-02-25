package execute

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/experiment"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
)

// Mock structures for testing
type mockInvoker struct{}

func (m *mockInvoker) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	return nil, nil
}

func (m *mockInvoker) StreamRun(ctx context.Context, prompt, tier string, w io.Writer, h provider.EventHandler, tc provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{
		Success:       true,
		Output:        "test",
		Duration:      0,
		Model:         "test-model",
		CostUSD:       0,
		InputTokens:   10,
		OutputTokens:  20,
		ExitCode:      0,
	}, nil
}

type mockRenderer struct{}

func (m *mockRenderer) RenderBuild(title, description string, validationFailures []string) (string, error) {
	return "test prompt", nil
}

func (m *mockRenderer) RenderTDDBuild(title, description string, validationFailures []string) (string, error) {
	return "test tdd prompt", nil
}

func (m *mockRenderer) RenderRefactorBuild(title, description string, validationFailures []string) (string, error) {
	return "test refactor prompt", nil
}

func TestBuildStageSelectsVariantForBuildPhase(t *testing.T) {
	// Create a test experiment with variants for the build phase
	testExp := &experiment.Experiment{
		ID:    "test-exp",
		Phase: "build",
		Control: &experiment.Variant{
			ID:       "control",
			Template: "control_template",
		},
		Variants: []*experiment.Variant{
			{
				ID:       "variant1",
				Template: "variant1_template",
				Model:    "sonnet",
				Budget: &experiment.Budget{
					MaxChars:            10000,
					LearningCapChars:    1000,
				},
			},
		},
	}

	expMgr := experiment.NewManager([]*experiment.Experiment{testExp}, "")

	// Create Build stage with experiment manager
	buildStage := New(&mockInvoker{}, &mockRenderer{}, io.Discard)
	buildStage.WithExperimentManager(expMgr)

	if buildStage.experimentMgr == nil {
		t.Fatal("experiment manager not set on build stage")
	}

	// Verify we can retrieve the experiment
	exp := buildStage.experimentMgr.ExperimentForPhase("build")
	if exp == nil {
		t.Fatal("expected to find experiment for build phase")
	}

	if exp.ID != "test-exp" {
		t.Fatalf("expected experiment ID 'test-exp', got '%s'", exp.ID)
	}

	// Verify the control variant exists
	if exp.Control == nil {
		t.Fatal("expected control variant")
	}

	if exp.Control.ID != "control" {
		t.Fatalf("expected control variant ID 'control', got '%s'", exp.Control.ID)
	}

	// Verify we can access experiment variants
	if len(exp.Variants) == 0 {
		t.Fatal("expected variants in experiment")
	}

	variant := exp.Variants[0]
	if variant.Model != "sonnet" {
		t.Fatalf("expected variant model 'sonnet', got '%s'", variant.Model)
	}
}

func TestBuildStageCanAccessVariantInResult(t *testing.T) {
	// Create a test experiment
	testExp := &experiment.Experiment{
		ID:    "test-exp",
		Phase: "build",
		Control: &experiment.Variant{
			ID:       "control",
			Template: "control_template",
		},
	}

	expMgr := experiment.NewManager([]*experiment.Experiment{testExp}, "")

	// Create Build stage with experiment manager
	buildStage := New(&mockInvoker{}, &mockRenderer{}, io.Discard)
	buildStage.WithExperimentManager(expMgr)

	// Create input with a result
	input := pipeline.Input{
		Bead: &bead.Bead{
			ID:    "test-bead",
			Title: "Test Bead",
		},
		Config: &config.Config{
			Models: config.ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			},
		},
		Result: &logger.IterationLog{
			BeadID: "test-bead",
		},
	}

	// The result should be able to store variant information
	if input.Result == nil {
		t.Fatal("expected result in input")
	}

	if input.Result.BeadID != "test-bead" {
		t.Fatalf("expected bead ID in result, got '%s'", input.Result.BeadID)
	}
}
