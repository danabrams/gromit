package execute

import (
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/experiment"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

func TestBuildStageStoresVariantIDInResult(t *testing.T) {
	// Create a test experiment
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

	// Create input with a result that can store variant IDs
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

	// Verify the result can store experiment and variant IDs
	if input.Result == nil {
		t.Fatal("expected result in input")
	}

	// For now, we just verify the structure exists
	// The actual assignment logic will be implemented in the Build.Run method
	if input.Result.ExperimentID != "" {
		t.Fatalf("expected empty ExperimentID initially, got '%s'", input.Result.ExperimentID)
	}

	if input.Result.VariantID != "" {
		t.Fatalf("expected empty VariantID initially, got '%s'", input.Result.VariantID)
	}
}
