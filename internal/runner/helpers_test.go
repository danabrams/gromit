package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestValidationGoCacheEnvUsesAbsolutePaths(t *testing.T) {
	env := validationGoCacheEnv(".")
	if len(env) == 0 {
		t.Fatal("validationGoCacheEnv returned empty env")
	}

	found := map[string]bool{
		"GOCACHE":    false,
		"GOMODCACHE": false,
		"GOPATH":     false,
	}

	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := parts[0], parts[1]
		if _, ok := found[key]; !ok {
			continue
		}
		found[key] = true
		if !filepath.IsAbs(value) {
			t.Fatalf("%s path must be absolute, got %q", key, value)
		}
	}

	for key, ok := range found {
		if !ok {
			t.Fatalf("missing %s entry", key)
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
	if exitCode != execFailureExitCode {
		t.Fatalf("exitCode = %d, want %d", exitCode, execFailureExitCode)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestIsInteractiveStdin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		statFn func() (os.FileInfo, error)
		want   bool
	}{
		{
			name: "interactive tty",
			statFn: func() (os.FileInfo, error) {
				return staticFileInfo{mode: os.ModeCharDevice}, nil
			},
			want: true,
		},
		{
			name: "piped stdin is non-interactive",
			statFn: func() (os.FileInfo, error) {
				return staticFileInfo{mode: os.ModeNamedPipe}, nil
			},
			want: false,
		},
		{
			name: "redirected file stdin is non-interactive",
			statFn: func() (os.FileInfo, error) {
				return staticFileInfo{mode: 0}, nil
			},
			want: false,
		},
		{
			name: "stat error",
			statFn: func() (os.FileInfo, error) {
				return nil, errors.New("stat failed")
			},
			want: false,
		},
		{
			name: "nil stat function",
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isInteractiveStdin(tt.statFn); got != tt.want {
				t.Fatalf("isInteractiveStdin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseYesNoResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "y", input: "y", want: true},
		{name: "yes", input: "yes", want: true},
		{name: "uppercase yes", input: "YES", want: true},
		{name: "surrounded whitespace", input: "  y  ", want: true},
		{name: "n", input: "n", want: false},
		{name: "no", input: "no", want: false},
		{name: "empty", input: "", want: false},
		{name: "whitespace empty", input: " \n\t ", want: false},
		{name: "unexpected value", input: "maybe", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseYesNoResponse(tt.input); got != tt.want {
				t.Fatalf("parseYesNoResponse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

type staticFileInfo struct {
	mode os.FileMode
}

func (s staticFileInfo) Name() string       { return "stdin" }
func (s staticFileInfo) Size() int64        { return 0 }
func (s staticFileInfo) Mode() os.FileMode  { return s.mode }
func (s staticFileInfo) ModTime() time.Time { return time.Time{} }
func (s staticFileInfo) IsDir() bool        { return false }
func (s staticFileInfo) Sys() interface{}   { return nil }
