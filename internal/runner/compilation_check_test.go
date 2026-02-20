package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func newCompilationCheckRunner(
	t *testing.T,
	compileCommand string,
	runCmd func(ctx context.Context, command string, workDir string) (string, string, int, error),
) (*Runner, *runtypes.BeadContext) {
	t.Helper()

	r := &Runner{
		cfg: &config.Config{
			Preflight: config.PreflightConfig{CompileCommand: compileCommand},
		},
		cmdRunnerFn: runCmd,
	}

	bc := &runtypes.BeadContext{
		BuildPrompt: "original prompt",
		Result:      &runtypes.IterationResult{},
	}

	return r, bc
}

func TestRunCompilationCheck_ErrorsAppendedToPrompt(t *testing.T) {
	const compileCommand = "go build ./cmd/gromit"
	executedCommand := ""

	r, bc := newCompilationCheckRunner(
		t,
		compileCommand,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			executedCommand = command
			return "", "internal/foo/bar.go:10: undefined: SomeSymbol", 1, nil
		},
	)

	r.runCompilationCheck(context.Background(), bc)

	if !strings.Contains(bc.BuildPrompt, "<compilation-errors>") {
		t.Fatal("expected compilation errors section in prompt")
	}
	if !strings.Contains(bc.BuildPrompt, "undefined: SomeSymbol") {
		t.Fatal("expected error output in prompt")
	}
	if !bc.Result.CompilationErrors {
		t.Fatal("expected CompilationErrors flag to be true")
	}
	if executedCommand != compileCommand {
		t.Fatalf("expected compile command %q, got %q", compileCommand, executedCommand)
	}
}

func TestRunCompilationCheck_NoErrors(t *testing.T) {
	const compileCommand = "go build ./..."
	executedCommand := ""

	r, bc := newCompilationCheckRunner(
		t,
		compileCommand,
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			executedCommand = command
			return "", "", 0, nil
		},
	)

	r.runCompilationCheck(context.Background(), bc)

	if bc.BuildPrompt != "original prompt" {
		t.Fatalf("expected prompt unchanged, got %q", bc.BuildPrompt)
	}
	if bc.Result.CompilationErrors {
		t.Fatal("expected CompilationErrors flag to be false")
	}
	if executedCommand != compileCommand {
		t.Fatalf("expected compile command %q, got %q", compileCommand, executedCommand)
	}
}

func TestRunCompilationCheck_Disabled(t *testing.T) {
	r, bc := newCompilationCheckRunner(
		t,
		"",
		func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			t.Fatal("should not be called when disabled")
			return "", "", 0, nil
		},
	)

	r.runCompilationCheck(context.Background(), bc)

	if bc.BuildPrompt != "original prompt" {
		t.Fatal("expected prompt unchanged when disabled")
	}
}
