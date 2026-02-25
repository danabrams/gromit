package execute

import (
	"testing"

	"github.com/danabrams/gromit/internal/experiment"
)

func TestBuildStageCanUseExperimentManager(t *testing.T) {
	// Create a test experiment with a variant for the build phase
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
	buildExp := expMgr.ExperimentForPhase("build")

	if buildExp == nil {
		t.Fatal("expected to find experiment for build phase")
	}

	if buildExp.ID != "test-exp" {
		t.Fatalf("expected experiment ID 'test-exp', got '%s'", buildExp.ID)
	}

	if len(buildExp.Variants) == 0 {
		t.Fatal("expected at least one variant in experiment")
	}

	variant := buildExp.Variants[0]
	if variant.Model != "sonnet" {
		t.Fatalf("expected variant model 'sonnet', got '%s'", variant.Model)
	}
}
