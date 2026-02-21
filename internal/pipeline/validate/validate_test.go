package validate

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// fakeCommandRunner is a test double for CommandRunner.
// runFn receives the call number (1-based) to allow scripted behavior per call.
type fakeCommandRunner struct {
	calls []runCall
	runFn func(callN int, command, workDir string) (string, string, int, error)
}

type runCall struct {
	command string
	workDir string
}

func (f *fakeCommandRunner) Run(ctx context.Context, command, workDir string) (string, string, int, error) {
	f.calls = append(f.calls, runCall{command: command, workDir: workDir})
	if f.runFn != nil {
		return f.runFn(len(f.calls), command, workDir)
	}
	return "", "", 0, nil
}

// fakeAutoFixer is a test double for AutoFixer.
type fakeAutoFixer struct {
	called bool
	fixErr error
}

func (f *fakeAutoFixer) Fix() error {
	f.called = true
	return f.fixErr
}

func makeInput(cfg *config.Config, iteration int) pipeline.Input {
	return pipeline.Input{
		Bead:      &bead.Bead{ID: "test-1", Title: "Test bead"},
		Config:    cfg,
		Iteration: iteration,
		Deadline:  time.Now().Add(time.Minute),
	}
}

func makeValidationConfig(commands []string) *config.Config {
	return &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  true,
			Commands: commands,
		},
	}
}

// TestValidate_PassesImmediately_ReturnsProceed verifies that when all validation
// commands pass on the first run, the stage returns Proceed with no ValidationFailures.
func TestValidate_PassesImmediately_ReturnsProceed(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := New(runner, io.Discard)

	cfg := makeValidationConfig([]string{"go test ./..."})
	in := makeInput(cfg, 1)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
	if len(out.ValidationFailures) != 0 {
		t.Errorf("ValidationFailures = %v, want empty", out.ValidationFailures)
	}
}

// TestValidate_AutoFix_Success_ReturnsProceed verifies AC: auto-fix success.
// When validation fails initially but auto-fix resolves the failure, the stage
// returns Proceed with no ValidationFailures and the auto-fixer was called.
func TestValidate_AutoFix_Success_ReturnsProceed(t *testing.T) {
	callCount := 0
	runner := &fakeCommandRunner{
		runFn: func(callN int, command, workDir string) (string, string, int, error) {
			callCount++
			if callCount == 1 {
				// First call: fail (formatting error)
				return "", "formatting error in foo.go", 1, nil
			}
			// Subsequent calls: pass (after auto-fix)
			return "", "", 0, nil
		},
	}
	fixer := &fakeAutoFixer{}
	stage := New(runner, io.Discard).WithAutoFixer(fixer)

	cfg := makeValidationConfig([]string{"go test ./..."})
	in := makeInput(cfg, 1)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}
	if len(out.ValidationFailures) != 0 {
		t.Errorf("ValidationFailures = %v, want empty after successful auto-fix", out.ValidationFailures)
	}
	if !fixer.called {
		t.Error("AutoFixer.Fix() was not called; want called on validation failure")
	}
}

// TestValidate_AutoFix_StillFailing_ReturnsValidationFailures verifies AC: auto-fix+still-failing.
// When validation fails and auto-fix does not resolve the failure, the stage returns
// Proceed with ValidationFailures populated (to be fed into the next Build stage).
func TestValidate_AutoFix_StillFailing_ReturnsValidationFailures(t *testing.T) {
	runner := &fakeCommandRunner{
		runFn: func(callN int, command, workDir string) (string, string, int, error) {
			// Always fail
			return "", "FAIL: TestFoo (0.01s)", 1, nil
		},
	}
	fixer := &fakeAutoFixer{}
	stage := New(runner, io.Discard).WithAutoFixer(fixer)

	cfg := makeValidationConfig([]string{"go test ./..."})
	in := makeInput(cfg, 1)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed (failures fed to next Build via ValidationFailures)", out.Decision)
	}
	if len(out.ValidationFailures) == 0 {
		t.Error("ValidationFailures is empty; want failures populated when auto-fix does not resolve")
	}
	if !fixer.called {
		t.Error("AutoFixer.Fix() was not called; want called on validation failure")
	}
}

// TestValidate_PeriodicFullValidation_UsesFullCommandsAtFrequency verifies AC: periodic validation gate.
// When Iteration % FullValidationEveryN == 0, the full command set is used instead of fast commands.
func TestValidate_PeriodicFullValidation_UsesFullCommandsAtFrequency(t *testing.T) {
	var commandsRun []string
	runner := &fakeCommandRunner{
		runFn: func(callN int, command, workDir string) (string, string, int, error) {
			commandsRun = append(commandsRun, command)
			return "", "", 0, nil
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			FastCommands:         []string{"go test ./internal/..."},
			FullCommands:         []string{"go test ./...", "go vet ./..."},
			FullValidationEveryN: 5,
		},
	}

	// At iteration 5 (multiple of 5), full commands should be used.
	in := makeInput(cfg, 5)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed", out.Decision)
	}

	// Verify full commands were used, not fast.
	foundFull := false
	for _, cmd := range commandsRun {
		if cmd == "go test ./..." {
			foundFull = true
		}
		if cmd == "go test ./internal/..." {
			t.Error("fast command was run; want full commands at frequency boundary")
		}
	}
	if !foundFull {
		t.Errorf("full commands not run; commandsRun = %v", commandsRun)
	}
}

// TestValidate_PeriodicFullValidation_UsesFastCommandsBetweenBoundaries verifies that
// between frequency boundaries, the fast command set is used.
func TestValidate_PeriodicFullValidation_UsesFastCommandsBetweenBoundaries(t *testing.T) {
	var commandsRun []string
	runner := &fakeCommandRunner{
		runFn: func(callN int, command, workDir string) (string, string, int, error) {
			commandsRun = append(commandsRun, command)
			return "", "", 0, nil
		},
	}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:              true,
			FastCommands:         []string{"go test ./internal/..."},
			FullCommands:         []string{"go test ./...", "go vet ./..."},
			FullValidationEveryN: 5,
		},
	}

	// At iteration 3 (not a multiple of 5), fast commands should be used.
	in := makeInput(cfg, 3)

	_, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	foundFast := false
	for _, cmd := range commandsRun {
		if cmd == "go test ./internal/..." {
			foundFast = true
		}
		if cmd == "go test ./..." {
			t.Error("full command was run at non-boundary iteration; want fast commands")
		}
	}
	if !foundFast {
		t.Errorf("fast commands not run; commandsRun = %v", commandsRun)
	}
}

// TestValidate_MandatoryCommandPrefixMissing_BlocksWithFailure verifies that when
// the configured commands do not include a mandatory command prefix, the stage returns
// Block with a failure message describing the missing prefix.
func TestValidate_MandatoryCommandPrefixMissing_BlocksWithFailure(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:           true,
			Commands:          []string{"go build ./..."},
			MandatoryCommands: []string{"go test"},
		},
	}
	in := makeInput(cfg, 1)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Block {
		t.Errorf("Decision = %v, want Block when mandatory prefix is missing", out.Decision)
	}
	if len(out.ValidationFailures) == 0 {
		t.Error("ValidationFailures is empty; want error describing missing mandatory prefix")
	}
	// Verify no commands were run (policy violation detected before execution).
	if len(runner.calls) != 0 {
		t.Errorf("commands were run despite policy violation; calls = %v", runner.calls)
	}
}

// TestValidate_MandatoryCommandPrefixPresent_ProceedsNormally verifies that when
// the configured commands satisfy all mandatory prefix requirements, the stage
// does not block and executes commands normally.
func TestValidate_MandatoryCommandPrefixPresent_ProceedsNormally(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:           true,
			Commands:          []string{"go test ./...", "go vet ./..."},
			MandatoryCommands: []string{"go test"},
		},
	}
	in := makeInput(cfg, 1)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed when mandatory prefix is satisfied", out.Decision)
	}
	if len(runner.calls) == 0 {
		t.Error("no commands were run; want commands executed when policy is satisfied")
	}
}

// TestValidate_Disabled_ReturnsProceedWithoutRunningCommands verifies that when
// validation is disabled in config, the stage returns Proceed without running any commands.
func TestValidate_Disabled_ReturnsProceedWithoutRunningCommands(t *testing.T) {
	runner := &fakeCommandRunner{}
	stage := New(runner, io.Discard)

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled:  false,
			Commands: []string{"go test ./..."},
		},
	}
	in := makeInput(cfg, 1)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if out.Decision != pipeline.Proceed {
		t.Errorf("Decision = %v, want Proceed when validation is disabled", out.Decision)
	}
	if len(runner.calls) != 0 {
		t.Errorf("commands were run when validation disabled; calls = %v", runner.calls)
	}
}

// TestValidate_NoAutoFixer_StillFailing_ReturnsValidationFailures verifies that when
// no auto-fixer is configured and validation fails, ValidationFailures are returned.
func TestValidate_NoAutoFixer_StillFailing_ReturnsValidationFailures(t *testing.T) {
	runner := &fakeCommandRunner{
		runFn: func(callN int, command, workDir string) (string, string, int, error) {
			return "", "FAIL: TestBar", 1, nil
		},
	}
	stage := New(runner, io.Discard) // no auto-fixer

	cfg := makeValidationConfig([]string{"go test ./..."})
	in := makeInput(cfg, 1)

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if len(out.ValidationFailures) == 0 {
		t.Error("ValidationFailures is empty; want failures when validation fails with no auto-fixer")
	}
}
