package runner

import (
	"context"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestBuildSpecGate_RunTestsUsesArgv(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var gotProgram string
	var gotArgs []string
	r := &Runner{
		cfg:      cfg,
		renderer: &mockPromptRenderer{},
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			gotProgram = program
			gotArgs = append([]string(nil), args...)
			return "tests ok", "", 0, nil
		},
	}

	gate, err := r.buildSpecGate()
	if err != nil {
		t.Fatalf("buildSpecGate() error: %v", err)
	}
	if _, err := gate.RunTests(context.Background()); err != nil {
		t.Fatalf("RunTests() error: %v", err)
	}
	if gotProgram != "go" {
		t.Fatalf("program = %q, want %q", gotProgram, "go")
	}
	if !reflect.DeepEqual(gotArgs, []string{"test", "-tags", "acceptance", "./..."}) {
		t.Fatalf("args = %#v, want %#v", gotArgs, []string{"test", "-tags", "acceptance", "./..."})
	}
}

func TestBuildSpecGate_GetDiffUsesArgv(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	var gotProgram string
	var gotArgs []string
	r := &Runner{
		cfg:      cfg,
		renderer: &mockPromptRenderer{},
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			gotProgram = program
			gotArgs = append([]string(nil), args...)
			return "diff", "", 0, nil
		},
	}

	gate, err := r.buildSpecGate()
	if err != nil {
		t.Fatalf("buildSpecGate() error: %v", err)
	}
	if _, err := gate.GetDiff(context.Background()); err != nil {
		t.Fatalf("GetDiff() error: %v", err)
	}
	if gotProgram != "git" {
		t.Fatalf("program = %q, want %q", gotProgram, "git")
	}
	if !reflect.DeepEqual(gotArgs, []string{"diff"}) {
		t.Fatalf("args = %#v, want %#v", gotArgs, []string{"diff"})
	}
}

func TestRunSpecGateCommand_UsesShellRunner(t *testing.T) {
	var gotCommand string
	r := &Runner{
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			gotCommand = command
			return "ok", "", 0, nil
		},
	}

	if _, err := r.runSpecGateCommand(context.Background(), "echo from config"); err != nil {
		t.Fatalf("runSpecGateCommand() error: %v", err)
	}
	if gotCommand != "echo from config" {
		t.Fatalf("command = %q, want %q", gotCommand, "echo from config")
	}
}
