package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestRunCompilationCheck_ErrorsAppendedToPrompt(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Preflight: config.PreflightConfig{CompileCommand: "go build ./..."},
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			if strings.Contains(command, "go build") {
				return "", "internal/foo/bar.go:10: undefined: SomeSymbol", 1, nil
			}
			return "", "", 0, nil
		},
	}

	bc := &runtypes.BeadContext{
		BuildPrompt: "original prompt",
		Result:      &runtypes.IterationResult{},
	}

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
}

func TestRunCompilationCheck_NoErrors(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Preflight: config.PreflightConfig{CompileCommand: "go build ./..."},
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "", 0, nil
		},
	}

	bc := &runtypes.BeadContext{
		BuildPrompt: "original prompt",
		Result:      &runtypes.IterationResult{},
	}

	r.runCompilationCheck(context.Background(), bc)

	if bc.BuildPrompt != "original prompt" {
		t.Fatalf("expected prompt unchanged, got %q", bc.BuildPrompt)
	}
	if bc.Result.CompilationErrors {
		t.Fatal("expected CompilationErrors flag to be false")
	}
}

func TestRunCompilationCheck_Disabled(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Preflight: config.PreflightConfig{CompileCommand: ""},
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			t.Fatal("should not be called when disabled")
			return "", "", 0, nil
		},
	}

	bc := &runtypes.BeadContext{
		BuildPrompt: "original prompt",
		Result:      &runtypes.IterationResult{},
	}

	r.runCompilationCheck(context.Background(), bc)

	if bc.BuildPrompt != "original prompt" {
		t.Fatal("expected prompt unchanged when disabled")
	}
}
