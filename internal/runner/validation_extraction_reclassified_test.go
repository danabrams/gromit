package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// TestValidationExtractionReclassified_RunValidationDelegation verifies that
// the Runner's runValidation method delegates command execution to the
// validation.Runner rather than executing validation logic inline. This is the
// unit-level version of the behavior previously verified in acceptance tests.
func TestValidationExtractionReclassified_RunValidationDelegation(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 0,
		},
		Claude: config.ClaudeConfig{
			BeadTimeout:     300,
			AnalysisTimeout: 60,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var buf strings.Builder

	// Wire a validation.Runner with an observable cmdRunner to confirm delegation.
	cmdRunnerCalled := false
	valRunner := validation.NewRunner(cfg,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			cmdRunnerCalled = true
			return "ok", "", 0, nil
		},
		nil, nil,
	)

	r := &Runner{
		cfg:              cfg,
		output:           &buf,
		analyzer:         &mockFailureAnalyzer{},
		validationRunner: valRunner,
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-delegation", Title: "Test", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	_ = r.runValidation(context.Background(), bc)

	if !cmdRunnerCalled {
		t.Error("runValidation must delegate command execution to the validation.Runner; " +
			"the injected cmdRunner was never called")
	}
}
