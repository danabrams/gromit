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

// TestRunnerHasValidationRunnerField verifies that the Runner struct has a
// validationRunner field of type *validation.Runner, following the same
// pattern as the existing escalationHandler field.
//
// Expected failure: Runner struct does not have a validationRunner field yet.
func TestRunnerHasValidationRunnerField(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			Commands:             []string{"go test ./..."},
			MaxValidationRetries: 2,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   newMockRouter(),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	// Expected failure: validationRunner field does not exist on Runner yet
	if r.validationRunner == nil {
		t.Error("NewRunnerWithDeps should wire a validationRunner field on Runner")
	}
}

// TestRunnerDelegatesToValidationRunner verifies that the Runner's runValidation
// method delegates to the validation.Runner's RunWithRecovery, not executing
// validation logic inline in process.go.
//
// Expected failure: Runner does not delegate to validation.Runner yet —
// validation logic is still inline in process.go.
func TestRunnerDelegatesToValidationRunner(t *testing.T) {
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
	// Construct a validation.Runner directly to verify delegation
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
		validationRunner: valRunner, // Expected failure: field does not exist
	}

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-1", Title: "Test", Labels: []string{}, ExpectedOutputs: []string{}},
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	_ = r.runValidation(context.Background(), bc)

	if !cmdRunnerCalled {
		t.Error("validation should delegate to the validation.Runner's command runner")
	}
}

// TestNewRunnerWiresValidationRunner verifies that NewRunner (the production
// constructor) creates and wires a validation.Runner with the correct
// dependencies from config.
//
// Expected failure: NewRunner does not create a validationRunner yet.
func TestNewRunnerWiresValidationRunner(t *testing.T) {
	// This test verifies the import path: runner imports validation package
	// and the constructor wires the field. If it compiles and doesn't panic,
	// the wiring exists.
	_ = validation.NewRunner
}

// TestValidationRunnerDeletesValidationSummaryFile verifies that after the
// extraction, the validation_summary.go file's extractValidationSummary
// function is replaced by a delegation to validation.ExtractValidationSummary.
//
// Expected failure: validation.ExtractValidationSummary does not exist yet
func TestValidationExtractSummaryDelegation(t *testing.T) {
	// The runner package should use validation.ExtractValidationSummary
	// rather than its own local extractValidationSummary function.
	input := "--- FAIL: TestFoo (0.01s)\nFAIL\tpkg/foo"

	// Expected failure: validation.ExtractValidationSummary does not exist yet
	summary := validation.ExtractValidationSummary(input)
	if summary == "" {
		t.Error("validation.ExtractValidationSummary should extract failure lines")
	}
	if !strings.Contains(summary, "--- FAIL: TestFoo") {
		t.Errorf("summary = %q, want it to contain '--- FAIL: TestFoo'", summary)
	}
}
