package runner

import (
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestDepsHasPolicyFields(t *testing.T) {
	depsType := reflect.TypeOf(Deps{})
	fields := []string{"EscalationPolicy", "MethodologyPolicy", "ValidationPolicy", "StuckPolicy", "ArgvRunner"}
	for _, field := range fields {
		if _, ok := depsType.FieldByName(field); !ok {
			t.Errorf("Deps missing %s field", field)
		}
	}
}

func TestNewRunnerWithDepsImplUsesInjectedArgvRunner(t *testing.T) {
	t.Parallel()

	called := false
	custom := func(_ context.Context, program string, args []string, workDir string) (string, string, int, error) {
		called = true
		if program != "tool" {
			t.Fatalf("program = %q, want %q", program, "tool")
		}
		if !reflect.DeepEqual(args, []string{"a", "b"}) {
			t.Fatalf("args = %v, want %v", args, []string{"a", "b"})
		}
		if workDir != "/tmp/work" {
			t.Fatalf("workDir = %q, want %q", workDir, "/tmp/work")
		}
		return "ok", "warn", 3, nil
	}

	r, err := newRunnerWithDepsImpl(&config.Config{}, io.Discard, t.TempDir(), Deps{
		ArgvRunner: custom,
		Renderer:   &mockPromptRenderer{},
	})
	if err != nil {
		t.Fatalf("newRunnerWithDepsImpl error: %v", err)
	}

	stdout, stderr, exitCode, runErr := r.runArgv(context.Background(), "tool", []string{"a", "b"}, "/tmp/work")
	if runErr != nil {
		t.Fatalf("runArgv error: %v", runErr)
	}
	if !called {
		t.Fatal("expected custom argv runner to be called")
	}
	if stdout != "ok" || stderr != "warn" || exitCode != 3 {
		t.Fatalf("runArgv = (%q, %q, %d), want (%q, %q, %d)", stdout, stderr, exitCode, "ok", "warn", 3)
	}
}

func TestNewRunnerWithDepsImplDefaultsArgvRunner(t *testing.T) {
	t.Parallel()

	r, err := newRunnerWithDepsImpl(&config.Config{}, io.Discard, t.TempDir(), Deps{Renderer: &mockPromptRenderer{}})
	if err != nil {
		t.Fatalf("newRunnerWithDepsImpl error: %v", err)
	}
	if r.argvRunnerFn == nil {
		t.Fatal("expected argvRunnerFn to default to non-nil")
	}
}
