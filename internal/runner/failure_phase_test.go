package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// TestExecuteBuildLoop_SetsFailurePhaseValidation verifies that executeBuildAndMethodologyLoop
// sets FailurePhase to the validation constant when validation fails after a successful build.
func TestExecuteBuildLoop_SetsFailurePhaseValidation(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Claude: config.ClaudeConfig{AnalysisTimeout: 30},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	failingCmd := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "FAIL\t./...", "", 1, nil
	}

	var buf strings.Builder
	sw := newSyncWriter(&buf)
	r := &Runner{
		cfg:              cfg,
		renderer:         &mockPromptRenderer{},
		analyzer:         &mockFailureAnalyzer{},
		output:           sw,
		syncOut:          sw,
		validationRunner: validation.NewRunner(cfg, failingCmd, nil, nil),
	}
	r.ensureMethodologyPolicy()

	b := newTestBead("fp-val-1", "Feature bead")
	bc := &runtypes.BeadContext{
		Bead:      b,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool { return true }, // build succeeds
	)

	if result.FailurePhase != failurephase.Validation {
		t.Errorf("FailurePhase = %q, want %q", result.FailurePhase, failurephase.Validation)
	}
}

// TestExecuteBuildLoop_SetsFailurePhaseBuild verifies that executeBuildAndMethodologyLoop
// sets FailurePhase to the build constant when executeWithRetry returns false.
func TestExecuteBuildLoop_SetsFailurePhaseBuild(t *testing.T) {
	r, _ := newMinimalRunnerForMethodology(t, nil, &mockPromptRenderer{})
	b := newTestBead("fp-build-1", "Feature bead")
	bc := newBeadContextForMethodology(b)

	result := r.executeBuildAndMethodologyLoop(
		context.Background(), bc,
		false, false,
		func() bool { return false }, // build always fails
	)

	if result.FailurePhase != failurephase.Build {
		t.Errorf("FailurePhase = %q, want %q", result.FailurePhase, failurephase.Build)
	}
}

func TestProcessBead_SetsFailurePhasePreflightOnSetupFailure(t *testing.T) {
	r := &Runner{}
	b := newTestBead("fp-preflight-1", "Feature bead")

	result := r.processBead(context.Background(), b, 1, time.Time{}, nil)

	if result.FailurePhase != failurephase.Preflight {
		t.Errorf("FailurePhase = %q, want %q", result.FailurePhase, failurephase.Preflight)
	}
}

func TestRunValidation_SetsFailurePhaseValidation(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
		Claude:    config.ClaudeConfig{AnalysisTimeout: 30},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	failingCmd := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "FAIL\t./...", 1, nil
	}

	var buf strings.Builder
	sw := newSyncWriter(&buf)
	r := &Runner{
		cfg:              cfg,
		renderer:         &mockPromptRenderer{},
		analyzer:         &mockFailureAnalyzer{},
		output:           sw,
		syncOut:          sw,
		validationRunner: validation.NewRunner(cfg, failingCmd, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead:      newTestBead("fp-run-val-1", "Feature bead"),
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	err := r.runValidation(context.Background(), bc)
	if err != errValidationFailed {
		t.Fatalf("runValidation() error = %v, want %v", err, errValidationFailed)
	}
	if bc.Result.FailurePhase != failurephase.Validation {
		t.Errorf("FailurePhase = %q, want %q", bc.Result.FailurePhase, failurephase.Validation)
	}
}

func TestRunValidation_SetsFailureCategoryFromAnalyzer(t *testing.T) {
	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: []string{"go test ./..."},
		},
		Preflight: config.PreflightConfig{},
		Claude:    config.ClaudeConfig{AnalysisTimeout: 30},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	failingCmd := func(ctx context.Context, command string, workDir string) (string, string, int, error) {
		return "", "FAIL\t./...", 1, nil
	}

	var buf strings.Builder
	sw := newSyncWriter(&buf)
	r := &Runner{
		cfg:      cfg,
		renderer: &mockPromptRenderer{},
		analyzer: &mockFailureAnalyzer{
			AnalyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
				return &analyzer.Analysis{
					Category:    analyzer.CategoryEnvironment,
					Recoverable: false,
					RootCause:   "toolchain mismatch",
				}, nil
			},
		},
		output:           sw,
		syncOut:          sw,
		validationRunner: validation.NewRunner(cfg, failingCmd, nil, nil),
	}

	bc := &runtypes.BeadContext{
		Bead:      newTestBead("fp-run-val-2", "Feature bead"),
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{WorkDir: t.TempDir()},
	}

	err := r.runValidation(context.Background(), bc)
	if err != errValidationFailed {
		t.Fatalf("runValidation() error = %v, want %v", err, errValidationFailed)
	}
	if bc.Result.FailureCategory != string(analyzer.CategoryEnvironment) {
		t.Errorf("FailureCategory = %q, want %q", bc.Result.FailureCategory, analyzer.CategoryEnvironment)
	}
}
