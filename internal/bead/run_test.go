package bead

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func assertRunArgsEqual(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("run() args len = %d, want %d; args=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("run() arg[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func writeExecutableScript(t *testing.T, script string) string {
	t.Helper()

	binaryPath := filepath.Join(t.TempDir(), "fake-bd")
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(%q): %v", binaryPath, err)
	}

	return binaryPath
}

func TestClientRun_UsesRunFnWhenSet(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	c := &Client{
		binary: filepath.Join(t.TempDir(), "does-not-exist"),
		Dir:    t.TempDir(),
		RunFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "from-runfn", nil
		},
	}

	out, err := c.run(context.Background(), "ready", "--json")
	if err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}
	if out != "from-runfn" {
		t.Fatalf("run() output = %q, want %q", out, "from-runfn")
	}

	wantArgs := []string{"ready", "--json"}
	assertRunArgsEqual(t, gotArgs, wantArgs)
}

func TestClientRun_SubprocessUsesConfiguredBinaryAndDir(t *testing.T) {
	t.Parallel()
	workDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"printf 'FAKE_BINARY\\n'\n" +
		"printf '%s\\n' \"$PWD\"\n" +
		"printf '%s\\n' \"$*\"\n"
	binaryPath := writeExecutableScript(t, script)

	c := &Client{
		binary: binaryPath,
		Dir:    workDir,
	}

	out, err := c.run(context.Background(), "ready", "--json", "--limit", "3")
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
	wantWorkDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q): %v", workDir, err)
	}
	if lines[1] != wantWorkDir {
		t.Fatalf("run() working dir = %q, want %q", lines[1], wantWorkDir)
	}
	if lines[2] != "ready --json --limit 3" {
		t.Fatalf("run() args = %q, want %q", lines[2], "ready --json --limit 3")
	}
}

func TestClientRun_SubprocessExitErrorWrapsStderr(t *testing.T) {
	t.Parallel()
	script := "#!/bin/sh\n" +
		"printf 'bd failed on stderr\\n' >&2\n" +
		"exit 17\n"
	binaryPath := writeExecutableScript(t, script)

	c := &Client{binary: binaryPath}

	out, err := c.run(context.Background(), "ready", "--json")
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
	t.Parallel()
	missingBinary := filepath.Join(t.TempDir(), "missing-bd")
	c := &Client{binary: missingBinary}

	out, err := c.run(context.Background(), "ready", "--json")
	if out != "" {
		t.Fatalf("run() output = %q, want empty string", out)
	}
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}

	// The error should mention the missing binary path (exec.ErrNotFound or similar).
	if !strings.Contains(err.Error(), missingBinary) && !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("run() error = %q, want to mention missing binary", err.Error())
	}
}

func TestClientRun_RetriesWithNoDBOnDatabaseNotFound(t *testing.T) {
	t.Parallel()
	counterPath := filepath.Join(t.TempDir(), "run-count")
	script := fmt.Sprintf(`#!/bin/sh
count=0
if [ -f %q ]; then
  count=$(cat %q)
fi
count=$((count + 1))
echo "$count" > %q
if [ "${BEADS_NO_DB:-}" = "true" ]; then
  printf '[{"id":"b1","title":"ok","description":"d","priority":2,"labels":[]}]'
  exit 0
fi
printf 'Error: failed to get ready work: Error 1049 (HY000): database not found: beads_gromit/\n' >&2
exit 1
`, counterPath, counterPath, counterPath)
	binaryPath := writeExecutableScript(t, script)

	c := &Client{binary: binaryPath}
	out, err := c.run(context.Background(), "ready", "--json", "--limit", "1")
	if err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}
	if !strings.Contains(out, `"id":"b1"`) {
		t.Fatalf("run() output = %q, want fallback success output", out)
	}

	countRaw, readErr := os.ReadFile(counterPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q): %v", counterPath, readErr)
	}
	if strings.TrimSpace(string(countRaw)) != "2" {
		t.Fatalf("run() invocation count = %q, want %q", strings.TrimSpace(string(countRaw)), "2")
	}
}

func TestClientRun_RetriesWithNoDBOnMissingIssuesTable(t *testing.T) {
	t.Parallel()
	counterPath := filepath.Join(t.TempDir(), "run-count")
	script := fmt.Sprintf(`#!/bin/sh
count=0
if [ -f %q ]; then
  count=$(cat %q)
fi
count=$((count + 1))
echo "$count" > %q
if [ "${BEADS_NO_DB:-}" = "true" ]; then
  printf '[{"id":"b1","title":"ok","description":"d","priority":2,"labels":[]}]'
  exit 0
fi
printf 'Error: failed to get ready work: Error 1146 (HY000): table not found: issues\n' >&2
exit 1
`, counterPath, counterPath, counterPath)
	binaryPath := writeExecutableScript(t, script)

	c := &Client{binary: binaryPath}
	out, err := c.run(context.Background(), "ready", "--json", "--limit", "1")
	if err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}
	if !strings.Contains(out, `"id":"b1"`) {
		t.Fatalf("run() output = %q, want fallback success output", out)
	}

	countRaw, readErr := os.ReadFile(counterPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q): %v", counterPath, readErr)
	}
	if strings.TrimSpace(string(countRaw)) != "2" {
		t.Fatalf("run() invocation count = %q, want %q", strings.TrimSpace(string(countRaw)), "2")
	}
}

func TestClientRun_ContextCancellationStopsCommand(t *testing.T) {
	t.Parallel()
	// Script that sleeps for 60 seconds - should be killed by context cancellation.
	script := "#!/bin/sh\nsleep 60\n"
	binaryPath := writeExecutableScript(t, script)

	c := &Client{
		binary:         binaryPath,
		CommandTimeout: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	_, err := c.run(ctx, "ready", "--json")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("run() should have returned an error for cancelled context")
	}

	// Should complete well under 60 seconds - the timeout is 100ms.
	if elapsed > 5*time.Second {
		t.Fatalf("run() took %v, expected it to be killed quickly by context timeout", elapsed)
	}
}

func TestClientRun_DefaultCommandTimeout(t *testing.T) {
	t.Parallel()
	c := &Client{binary: "bd"}
	if c.commandTimeout() != DefaultCommandTimeout {
		t.Fatalf("commandTimeout() = %v, want %v", c.commandTimeout(), DefaultCommandTimeout)
	}
}

func TestClientRun_CustomCommandTimeout(t *testing.T) {
	t.Parallel()
	c := &Client{binary: "bd", CommandTimeout: 5 * time.Second}
	if c.commandTimeout() != 5*time.Second {
		t.Fatalf("commandTimeout() = %v, want %v", c.commandTimeout(), 5*time.Second)
	}
}

func TestClientRunWithEnv_UsesProcutilLifecycle(t *testing.T) {
	script := "#!/bin/sh\n" +
		"printf 'FOO=%s\\n' \"$FOO\"\n" +
		"printf 'BAZ=%s\\n' \"$BAZ\"\n" +
		"printf 'ARGS=%s\\n' \"$*\"\n"
	binaryPath := writeExecutableScript(t, script)

	var waitCalled bool
	oldWait := waitForProcessCapacityFn
	t.Cleanup(func() { waitForProcessCapacityFn = oldWait })
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		waitCalled = true
		if maxWait <= 0 {
			t.Fatalf("expected positive maxWait, got %v", maxWait)
		}
		return nil
	}

	var killCalled bool
	oldKill := killDescendantsOnCancelFn
	t.Cleanup(func() { killDescendantsOnCancelFn = oldKill })
	killDescendantsOnCancelFn = func(ctx context.Context, cmd *exec.Cmd) {
		killCalled = true
	}

	var reapCalled bool
	oldReap := reapProcessTreeFn
	t.Cleanup(func() { reapProcessTreeFn = oldReap })
	reapProcessTreeFn = func(cmd *exec.Cmd) {
		reapCalled = true
	}

	var envCalled bool
	oldSubprocessEnv := subprocessEnvFn
	t.Cleanup(func() { subprocessEnvFn = oldSubprocessEnv })
	subprocessEnvFn = func() []string {
		envCalled = true
		return []string{"FOO=proc"}
	}

	c := &Client{binary: binaryPath}
	out, err := c.runWithEnv(context.Background(), []string{"print-env"}, []string{"BAZ=qux"})
	if err != nil {
		t.Fatalf("runWithEnv() error = %v", err)
	}

	if !waitCalled {
		t.Fatal("runWithEnv() did not wait for process capacity")
	}
	if !killCalled {
		t.Fatal("runWithEnv() did not call KillDescendantsOnCancel")
	}
	if !reapCalled {
		t.Fatal("runWithEnv() did not defer ReapProcessTree")
	}
	if !envCalled {
		t.Fatal("runWithEnv() did not use subprocess env")
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	got := make(map[string]string)
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			got[parts[0]] = parts[1]
		}
	}
	if got["FOO"] != "proc" {
		t.Fatalf("FOO = %q, want %q", got["FOO"], "proc")
	}
	if got["BAZ"] != "qux" {
		t.Fatalf("BAZ = %q, want %q", got["BAZ"], "qux")
	}
	if got["ARGS"] != "print-env" {
		t.Fatalf("ARGS = %q, want %q", got["ARGS"], "print-env")
	}
}

func TestClientRunWithEnvCombinedOutput_UsesProcutilLifecycle(t *testing.T) {
	script := "#!/bin/sh\n" +
		"printf 'FOO=%s\\n' \"$FOO\"\n" +
		"printf 'BAZ=%s\\n' \"$BAZ\"\n" +
		"printf 'ARGS=%s\\n' \"$*\"\n"
	binaryPath := writeExecutableScript(t, script)

	var waitCalled bool
	oldWait := waitForProcessCapacityFn
	t.Cleanup(func() { waitForProcessCapacityFn = oldWait })
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		waitCalled = true
		if maxWait <= 0 {
			t.Fatalf("expected positive maxWait, got %v", maxWait)
		}
		return nil
	}

	var killCalled bool
	oldKill := killDescendantsOnCancelFn
	t.Cleanup(func() { killDescendantsOnCancelFn = oldKill })
	killDescendantsOnCancelFn = func(ctx context.Context, cmd *exec.Cmd) {
		killCalled = true
	}

	var reapCalled bool
	oldReap := reapProcessTreeFn
	t.Cleanup(func() { reapProcessTreeFn = oldReap })
	reapProcessTreeFn = func(cmd *exec.Cmd) {
		reapCalled = true
	}

	var envCalled bool
	oldSubprocessEnv := subprocessEnvFn
	t.Cleanup(func() { subprocessEnvFn = oldSubprocessEnv })
	subprocessEnvFn = func() []string {
		envCalled = true
		return []string{"FOO=proc"}
	}

	c := &Client{binary: binaryPath}
	out, err := c.runWithEnvCombinedOutput(context.Background(), []string{"print-env"}, []string{"BAZ=qux"})
	if err != nil {
		t.Fatalf("runWithEnvCombinedOutput() error = %v", err)
	}

	if !waitCalled {
		t.Fatal("runWithEnvCombinedOutput() did not wait for process capacity")
	}
	if !killCalled {
		t.Fatal("runWithEnvCombinedOutput() did not call KillDescendantsOnCancel")
	}
	if !reapCalled {
		t.Fatal("runWithEnvCombinedOutput() did not defer ReapProcessTree")
	}
	if !envCalled {
		t.Fatal("runWithEnvCombinedOutput() did not use subprocess env")
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	got := make(map[string]string)
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			got[parts[0]] = parts[1]
		}
	}
	if got["FOO"] != "proc" {
		t.Fatalf("FOO = %q, want %q", got["FOO"], "proc")
	}
	if got["BAZ"] != "qux" {
		t.Fatalf("BAZ = %q, want %q", got["BAZ"], "qux")
	}
	if got["ARGS"] != "print-env" {
		t.Fatalf("ARGS = %q, want %q", got["ARGS"], "print-env")
	}
}

func TestClientRepoBaseName_UsesProcutilLifecycle(t *testing.T) {
	repoDir := t.TempDir()

	initCmd := exec.Command("git", "init")
	initCmd.Dir = repoDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	var waitCalled bool
	oldWait := waitForProcessCapacityFn
	t.Cleanup(func() { waitForProcessCapacityFn = oldWait })
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		waitCalled = true
		if maxWait <= 0 {
			t.Fatalf("expected positive maxWait, got %v", maxWait)
		}
		return nil
	}

	var reapCalled bool
	oldReap := reapProcessTreeFn
	t.Cleanup(func() { reapProcessTreeFn = oldReap })
	reapProcessTreeFn = func(cmd *exec.Cmd) {
		reapCalled = true
	}

	c := &Client{Dir: repoDir}
	got, err := c.repoBaseName()
	if err != nil {
		t.Fatalf("repoBaseName() error = %v", err)
	}
	if got != filepath.Base(repoDir) {
		t.Fatalf("repoBaseName() = %q, want %q", got, filepath.Base(repoDir))
	}
	if !waitCalled {
		t.Fatal("repoBaseName() did not wait for process capacity")
	}
	if !reapCalled {
		t.Fatal("repoBaseName() did not reap process tree")
	}
}
