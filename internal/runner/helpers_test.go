package runner

import (
	"context"
	"reflect"
	"testing"
)

func TestRunArgvUsesInjectedRunner(t *testing.T) {
	var gotProgram string
	var gotArgs []string
	var gotWorkDir string

	runner := &Runner{
		argvRunnerFn: func(ctx context.Context, program string, args []string, workDir string) (string, string, int, error) {
			gotProgram = program
			gotArgs = append([]string(nil), args...)
			gotWorkDir = workDir
			return "stdout", "stderr", 7, nil
		},
	}

	stdout, stderr, exitCode, err := runner.runArgv(context.Background(), "tool", []string{"a", "b"}, "/work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "stdout" {
		t.Fatalf("stdout = %q, want %q", stdout, "stdout")
	}
	if stderr != "stderr" {
		t.Fatalf("stderr = %q, want %q", stderr, "stderr")
	}
	if exitCode != 7 {
		t.Fatalf("exitCode = %d, want 7", exitCode)
	}
	if gotProgram != "tool" {
		t.Fatalf("program = %q, want %q", gotProgram, "tool")
	}
	if !reflect.DeepEqual(gotArgs, []string{"a", "b"}) {
		t.Fatalf("args = %#v, want %#v", gotArgs, []string{"a", "b"})
	}
	if gotWorkDir != "/work" {
		t.Fatalf("workDir = %q, want %q", gotWorkDir, "/work")
	}
}
