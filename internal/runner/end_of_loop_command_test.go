package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestRunEndOfLoopCommandEmptyNoOp(t *testing.T) {
	var buf strings.Builder
	called := false
	r := &Runner{
		cfg:    &config.Config{},
		output: &buf,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			called = true
			return "", "", 0, nil
		},
	}

	err := r.runEndOfLoopCommand()
	if err != nil {
		t.Fatalf("runEndOfLoopCommand() error = %v", err)
	}
	if called {
		t.Fatal("expected command runner not to be called for empty command")
	}
	if got := buf.String(); got != "" {
		t.Fatalf("expected no output for empty command, got %q", got)
	}
}

func TestRunEndOfLoopCommandWritesStdoutAndStderr(t *testing.T) {
	var buf strings.Builder
	called := false
	r := &Runner{
		cfg: &config.Config{
			Loop: config.LoopConfig{
				EndOfLoopCommand: "echo done",
			},
		},
		output: &buf,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			called = true
			if command != "echo done" {
				t.Fatalf("unexpected command: %q", command)
			}
			if workDir != "" {
				t.Fatalf("unexpected workDir: %q", workDir)
			}
			return "stdout text\n", "stderr text\n", 0, nil
		},
	}

	err := r.runEndOfLoopCommand()
	if err != nil {
		t.Fatalf("runEndOfLoopCommand() error = %v", err)
	}
	if !called {
		t.Fatal("expected command runner to be called")
	}

	output := buf.String()
	if !strings.Contains(output, "Running end-of-loop command: echo done") {
		t.Fatalf("expected command log line, got %q", output)
	}
	if !strings.Contains(output, "stdout text") {
		t.Fatalf("expected stdout in output, got %q", output)
	}
	if !strings.Contains(output, "stderr text") {
		t.Fatalf("expected stderr in output, got %q", output)
	}
}

func TestRunEndOfLoopCommandReturnsErrorOnNonZeroExit(t *testing.T) {
	var buf strings.Builder
	r := &Runner{
		cfg: &config.Config{
			Loop: config.LoopConfig{
				EndOfLoopCommand: "exit 12",
			},
		},
		output: &buf,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "stdout failure\n", "stderr failure\n", 12, nil
		},
	}

	err := r.runEndOfLoopCommand()
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if !strings.Contains(err.Error(), "end-of-loop command failed (exit 12): stderr failure") {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "stdout failure") {
		t.Fatalf("expected stdout in output, got %q", output)
	}
	if !strings.Contains(output, "stderr failure") {
		t.Fatalf("expected stderr in output, got %q", output)
	}
}

func TestRunEndOfLoopCommandReturnsRunnerError(t *testing.T) {
	expected := errors.New("runner boom")
	r := &Runner{
		cfg: &config.Config{
			Loop: config.LoopConfig{
				EndOfLoopCommand: "echo fail",
			},
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "", 0, expected
		},
	}

	err := r.runEndOfLoopCommand()
	if err == nil {
		t.Fatal("expected runner error")
	}
	if !strings.Contains(err.Error(), "end-of-loop command failed") {
		t.Fatalf("expected wrapped error message, got %v", err)
	}
	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped runner error, got %v", err)
	}
}

func TestRunEndOfLoopCommandNilRunnerAndConfig(t *testing.T) {
	var nilRunner *Runner
	if err := nilRunner.runEndOfLoopCommand(); err != nil {
		t.Fatalf("expected nil error for nil runner, got %v", err)
	}

	r := &Runner{}
	if err := r.runEndOfLoopCommand(); err != nil {
		t.Fatalf("expected nil error for nil config, got %v", err)
	}
}
