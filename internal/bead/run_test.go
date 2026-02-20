package bead

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientRun_UsesRunFnWhenSet(t *testing.T) {
	var gotArgs []string
	c := &Client{
		binary: filepath.Join(t.TempDir(), "does-not-exist"),
		Dir:    t.TempDir(),
		RunFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "from-runfn", nil
		},
	}

	out, err := c.run("ready", "--json")
	if err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}
	if out != "from-runfn" {
		t.Fatalf("run() output = %q, want %q", out, "from-runfn")
	}

	wantArgs := []string{"ready", "--json"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("run() args len = %d, want %d; args=%v", len(gotArgs), len(wantArgs), gotArgs)
	}
	for i, want := range wantArgs {
		if gotArgs[i] != want {
			t.Fatalf("run() arg[%d] = %q, want %q", i, gotArgs[i], want)
		}
	}
}

func TestClientRun_SubprocessUsesConfiguredBinaryAndDir(t *testing.T) {
	binDir := t.TempDir()
	workDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "fake-bd")

	script := "#!/bin/sh\n" +
		"printf 'FAKE_BINARY\\n'\n" +
		"printf '%s\\n' \"$PWD\"\n" +
		"printf '%s\\n' \"$*\"\n"

	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", binaryPath, err)
	}

	c := &Client{
		binary: binaryPath,
		Dir:    workDir,
	}

	out, err := c.run("ready", "--json", "--limit", "3")
	if err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("run() output lines = %d, want 3; output=%q", len(lines), out)
	}
	if lines[0] != "FAKE_BINARY" {
		t.Fatalf("run() did not use configured binary; marker = %q", lines[0])
	}
	if lines[1] != workDir {
		t.Fatalf("run() working dir = %q, want %q", lines[1], workDir)
	}
	if lines[2] != "ready --json --limit 3" {
		t.Fatalf("run() args = %q, want %q", lines[2], "ready --json --limit 3")
	}
}

func TestClientRun_SubprocessExitErrorWrapsStderr(t *testing.T) {
	binDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "fake-bd")

	script := "#!/bin/sh\n" +
		"printf 'bd failed on stderr\\n' >&2\n" +
		"exit 17\n"

	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", binaryPath, err)
	}

	c := &Client{binary: binaryPath}

	out, err := c.run("ready", "--json")
	if out != "" {
		t.Fatalf("run() output = %q, want empty string", out)
	}
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run() error does not wrap *exec.ExitError: %T %v", err, err)
	}

	errText := err.Error()
	if !strings.Contains(errText, "exit status 17") {
		t.Fatalf("run() error = %q, want to contain %q", errText, "exit status 17")
	}
	if !strings.Contains(errText, "bd failed on stderr\n") {
		t.Fatalf("run() error = %q, want to contain wrapped stderr text", errText)
	}
}

func TestClientRun_SubprocessNonExitErrorIsUnchanged(t *testing.T) {
	c := &Client{binary: filepath.Join(t.TempDir(), "missing-bd")}

	out, err := c.run("ready", "--json")
	if out != "" {
		t.Fatalf("run() output = %q, want empty string", out)
	}
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("run() error unexpectedly wraps *exec.ExitError: %v", err)
	}
}
