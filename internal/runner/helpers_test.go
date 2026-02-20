package runner

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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

func TestDefaultArgvRunnerSetsEnv(t *testing.T) {
	workDir := t.TempDir()
	stdout, stderr, exitCode, err := defaultArgvRunner(context.Background(), "env", nil, workDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, expected := range []string{
		"GIT_TERMINAL_PROMPT=0",
		"CI=1",
		"NONINTERACTIVE=1",
		"TERM=dumb",
		"GOCACHE=" + filepath.Join(workDir, ".gromit", "tmp", "go-build-cache"),
		"GOMODCACHE=" + filepath.Join(workDir, ".gromit", "tmp", "go-mod-cache"),
		"GOPATH=" + filepath.Join(workDir, ".gromit", "tmp", "go-path"),
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("stdout missing %q", expected)
		}
	}
}

func TestDefaultArgvRunnerSuccess(t *testing.T) {
	workDir := t.TempDir()
	stdout, stderr, exitCode, err := defaultArgvRunner(
		context.Background(),
		"sh",
		[]string{"-c", "printf 'hello'"},
		workDir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if stdout != "hello" {
		t.Fatalf("stdout = %q, want %q", stdout, "hello")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestDefaultArgvRunnerNonZeroExit(t *testing.T) {
	workDir := t.TempDir()
	stdout, stderr, exitCode, err := defaultArgvRunner(
		context.Background(),
		"sh",
		[]string{"-c", "echo boom >&2; exit 23"},
		workDir,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 23 {
		t.Fatalf("exitCode = %d, want 23", exitCode)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if strings.TrimSpace(stderr) != "boom" {
		t.Fatalf("stderr = %q, want %q", stderr, "boom")
	}
}

func TestDefaultArgvRunnerExecFailure(t *testing.T) {
	workDir := t.TempDir()
	stdout, stderr, exitCode, err := defaultArgvRunner(
		context.Background(),
		"definitely-not-a-real-executable",
		nil,
		workDir,
	)
	if err == nil {
		t.Fatalf("expected error for missing executable")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if exitCode != -1 {
		t.Fatalf("exitCode = %d, want -1", exitCode)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}
