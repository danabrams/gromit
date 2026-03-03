package bead

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/procutil"
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

func TestClientRunWithRunner_UsesRunFnWhenSet(t *testing.T) {
	t.Parallel()
	var gotArgs []string
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "runfn", nil
		},
	}

	runner := func(ctx context.Context, args []string, extraEnv []string) (string, error) {
		t.Fatalf("runner should not be called when RunFn is set")
		return "", nil
	}

	out, err := c.runWithRunner(context.Background(), []string{"close", "bd-1"}, nil, runner)
	if err != nil {
		t.Fatalf("runWithRunner() unexpected error: %v", err)
	}
	if out != "runfn" {
		t.Fatalf("runWithRunner() output = %q, want %q", out, "runfn")
	}
	assertRunArgsEqual(t, gotArgs, []string{"close", "bd-1"})
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

func TestClientRunWithEnvUsesProcutilDefaultWait(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("wait stub")
	var gotMaxWait time.Duration

	restore := restoreBeadProcutilFns(t)
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		gotMaxWait = maxWait
		return sentinel
	}

	c := &Client{binary: filepath.Join(t.TempDir(), "missing-bd")}
	_, err := c.runWithEnv(context.Background(), []string{"ready"}, nil)
	restore()
	if !errors.Is(err, sentinel) {
		t.Fatalf("runWithEnv() error = %v, want %v", err, sentinel)
	}
	if gotMaxWait != procutil.DefaultProcessCapacityMaxWait {
		t.Fatalf("runWithEnv() wait = %v, want %v", gotMaxWait, procutil.DefaultProcessCapacityMaxWait)
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

func TestClientRun_RetriesAfterJSONLSyncOutOfSync(t *testing.T) {
	t.Parallel()
	statePath := filepath.Join(t.TempDir(), "sync-state")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "init" ] && [ "$2" = "--from-jsonl" ]; then
  echo "synced" > %q
  exit 0
fi
if [ "$1" = "ready" ]; then
  if [ -f %q ]; then
    printf '[{"id":"b1","title":"ok","description":"d","priority":2,"labels":[]}]'
    exit 0
  fi
  printf 'Error: database out of sync: issues.jsonl is newer than last import (2026-03-02T14:55:16-05:00 > 2026-03-02T14:11:11-05:00)\n' >&2
  exit 1
fi
printf 'unexpected args: %%s\n' "$*" >&2
exit 1
`, statePath, statePath)
	binaryPath := writeExecutableScript(t, script)

	c := &Client{binary: binaryPath}
	out, err := c.run(context.Background(), "ready", "--json", "--limit", "1")
	if err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}
	if !strings.Contains(out, `"id":"b1"`) {
		t.Fatalf("run() output = %q, want fallback success output", out)
	}

	stateRaw, readErr := os.ReadFile(statePath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q): %v", statePath, readErr)
	}
	if strings.TrimSpace(string(stateRaw)) != "synced" {
		t.Fatalf("sync state = %q, want %q", strings.TrimSpace(string(stateRaw)), "synced")
	}
}

func TestClientRunClose_RetriesAfterJSONLSyncOutOfSync(t *testing.T) {
	t.Parallel()
	statePath := filepath.Join(t.TempDir(), "sync-state")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "init" ] && [ "$2" = "--from-jsonl" ]; then
  echo "synced" > %q
  exit 0
fi
if [ "$1" = "close" ]; then
  if [ -f %q ]; then
    printf 'closed\n'
    exit 0
  fi
  printf 'Error: database out of sync: issues.jsonl is newer than last import (2026-03-02T14:55:16-05:00 > 2026-03-02T14:11:11-05:00)\n' >&2
  exit 1
fi
printf 'unexpected args: %%s\n' "$*" >&2
exit 1
`, statePath, statePath)
	binaryPath := writeExecutableScript(t, script)

	c := &Client{binary: binaryPath}
	out, err := c.runClose(context.Background(), "bd-1")
	if err != nil {
		t.Fatalf("runClose() unexpected error: %v", err)
	}
	if !strings.Contains(out, "closed") {
		t.Fatalf("runClose() output = %q, want %q", out, "closed")
	}

	stateRaw, readErr := os.ReadFile(statePath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q): %v", statePath, readErr)
	}
	if strings.TrimSpace(string(stateRaw)) != "synced" {
		t.Fatalf("sync state = %q, want %q", strings.TrimSpace(string(stateRaw)), "synced")
	}
}

func TestClientRunAndRunCloseUseSharedRetryHelper(t *testing.T) {
	t.Parallel()

	var calls []string
	originalFn := runWithRetryCascadeFn
	runWithRetryCascadeFn = func(c *Client, ctx context.Context, args []string, extraEnv []string, runner func(context.Context, []string, []string) (string, error)) (string, error) {
		if len(args) == 0 {
			t.Fatalf("runWithRetryCascade called with no args")
		}
		calls = append(calls, args[0])
		return "", nil
	}
	defer func() { runWithRetryCascadeFn = originalFn }()

	c := &Client{binary: "bd"}
	if _, err := c.run(context.Background(), "ready"); err != nil {
		t.Fatalf("run() unexpected error: %v", err)
	}
	if _, err := c.runClose(context.Background(), "bd-1"); err != nil {
		t.Fatalf("runClose() unexpected error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("shared retry helper called %d times, want 2", len(calls))
	}
	if calls[0] != "ready" {
		t.Fatalf("1st call args[0] = %q, want %q", calls[0], "ready")
	}
	if calls[1] != "close" {
		t.Fatalf("2nd call args[0] = %q, want %q", calls[1], "close")
	}
}

func TestClientRunWithRetryCascade_RetriesWithNoDB(t *testing.T) {
	t.Parallel()

	var callCount int
	runner := func(ctx context.Context, args []string, extraEnv []string) (string, error) {
		callCount++
		switch callCount {
		case 1:
			if len(extraEnv) != 0 {
				t.Fatalf("first invocation should not include extra env, got %v", extraEnv)
			}
			return "", fmt.Errorf("database not found: beads_gromit")
		case 2:
			if len(extraEnv) != 1 || extraEnv[0] != "BEADS_NO_DB=true" {
				t.Fatalf("second invocation should receive BEADS_NO_DB=true, got %v", extraEnv)
			}
			return "recovered", nil
		default:
			t.Fatalf("unexpected invocation %d", callCount)
		}
		return "", fmt.Errorf("unreachable")
	}

	c := &Client{}
	out, err := c.runWithRetryCascade(context.Background(), []string{"ready"}, nil, runner)
	if err != nil {
		t.Fatalf("runWithRetryCascade() error = %v", err)
	}
	if out != "recovered" {
		t.Fatalf("runWithRetryCascade() output = %q, want %q", out, "recovered")
	}
}

func TestClientRunWithRetryCascade_PreservesExtraEnvDuringRetries(t *testing.T) {
	t.Parallel()

	extraEnv := []string{"FOO=bar"}
	var callCount int
	runner := func(ctx context.Context, args []string, env []string) (string, error) {
		callCount++
		copied := append([]string(nil), env...)
		if callCount == 1 {
			if !reflect.DeepEqual(copied, extraEnv) {
				t.Fatalf("first call env = %v, want %v", copied, extraEnv)
			}
			return "", fmt.Errorf("database not found: beads_gromit")
		}
		if callCount == 2 {
			want := append(append([]string(nil), extraEnv...), "BEADS_NO_DB=true")
			if !reflect.DeepEqual(copied, want) {
				t.Fatalf("second call env = %v, want %v", copied, want)
			}
			return "recovered", nil
		}
		t.Fatalf("unexpected runner invocation %d", callCount)
		return "", fmt.Errorf("unreachable")
	}

	c := &Client{}
	out, err := c.runWithRetryCascade(context.Background(), []string{"ready"}, extraEnv, runner)
	if err != nil {
		t.Fatalf("runWithRetryCascade() error = %v", err)
	}
	if out != "recovered" {
		t.Fatalf("runWithRetryCascade() output = %q, want %q", out, "recovered")
	}
}

func TestClientRunDeriveIssuePrefixUsesCallerContext(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", repoDir, err)
	}
	initCmd := exec.Command("git", "init")
	initCmd.Dir = repoDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	gitScriptDir := t.TempDir()
	gitScriptPath := filepath.Join(gitScriptDir, "git")
	gitScript := "#!/bin/sh\nsleep 1\nprintf '%s\\n' \"$PWD\"\n"
	if err := os.WriteFile(gitScriptPath, []byte(gitScript), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	origPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", gitScriptDir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	readyFile := filepath.Join(t.TempDir(), "ready-done")
	readyDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(readyFile); err == nil {
					cancel()
					close(readyDone)
					return
				}
			}
		}
	}()

	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "ready" ]; then
  printf 'issue_prefix config is missing\n' >&2
  touch %q
  exit 1
fi
printf 'unexpected args: %%s\n' "$*" >&2
exit 1
`, readyFile)
	binaryPath := writeExecutableScript(t, script)

	c := &Client{binary: binaryPath, Dir: repoDir}
	_, err := c.run(ctx, "ready", "--json")
	if err == nil {
		t.Fatal("run() error = nil, want cancellation error")
	}
	select {
	case <-readyDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ready marker")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("run() error = %v, want context cancellation", err)
	}
}

func TestClientRunCloseDeriveIssuePrefixUsesCallerContext(t *testing.T) {
	t.Parallel()
	repoDir := t.TempDir()
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", repoDir, err)
	}
	initCmd := exec.Command("git", "init")
	initCmd.Dir = repoDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "git")
	readyFile := filepath.Join(t.TempDir(), "close-ready")
	script := fmt.Sprintf(`#!/bin/sh
sleep 1
if [ "$1" = "close" ]; then
  printf 'issue_prefix config is missing\n' >&2
  touch %q
  exit 1
fi
printf 'unexpected args: %%s\n' "$*" >&2
exit 1
`, readyFile)
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	origPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", scriptDir+string(os.PathListSeparator)+origPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	readyDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := os.Stat(readyFile); err == nil {
					cancel()
					close(readyDone)
					return
				}
			}
		}
	}()

	c := &Client{binary: scriptPath, Dir: repoDir}
	_, err := c.runClose(ctx, "bd-1")
	if err == nil {
		t.Fatal("runClose() error = nil, want cancellation error")
	}
	select {
	case <-readyDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for close marker")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("runClose() error = %v, want context cancellation", err)
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
	restore := restoreBeadProcutilFns(t)
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		waitCalled = true
		if maxWait <= 0 {
			t.Fatalf("expected positive maxWait, got %v", maxWait)
		}
		return nil
	}

	var killCalled bool
	killDescendantsOnCancelFn = func(ctx context.Context, cmd *exec.Cmd) {
		killCalled = true
	}

	var reapCalled bool
	reapProcessTreeFn = func(cmd *exec.Cmd) {
		reapCalled = true
	}

	var envCalled bool
	subprocessEnvFn = func() []string {
		envCalled = true
		return []string{"FOO=proc"}
	}

	c := &Client{binary: binaryPath}
	out, err := c.runWithEnv(context.Background(), []string{"print-env"}, []string{"BAZ=qux"})
	restore()
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
	restore := restoreBeadProcutilFns(t)
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		waitCalled = true
		if maxWait <= 0 {
			t.Fatalf("expected positive maxWait, got %v", maxWait)
		}
		return nil
	}

	var killCalled bool
	killDescendantsOnCancelFn = func(ctx context.Context, cmd *exec.Cmd) {
		killCalled = true
	}

	var reapCalled bool
	reapProcessTreeFn = func(cmd *exec.Cmd) {
		reapCalled = true
	}

	var envCalled bool
	subprocessEnvFn = func() []string {
		envCalled = true
		return []string{"FOO=proc"}
	}

	c := &Client{binary: binaryPath}
	out, err := c.runWithEnvCombinedOutput(context.Background(), []string{"print-env"}, []string{"BAZ=qux"})
	restore()
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

func TestClientRunWithEnv_SetsCanonicalBeadsDirFromGitCommonDir(t *testing.T) {
	mainDir := t.TempDir()
	runInDir(t, mainDir, "git", "init")
	runInDir(t, mainDir, "git", "config", "user.email", "test@example.com")
	runInDir(t, mainDir, "git", "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(mainDir, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}
	runInDir(t, mainDir, "git", "add", "README.md")
	runInDir(t, mainDir, "git", "commit", "-m", "init")

	worktreeDir := filepath.Join(t.TempDir(), "wt")
	runInDir(t, mainDir, "git", "worktree", "add", worktreeDir)
	if err := os.MkdirAll(filepath.Join(mainDir, ".beads"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.beads): %v", err)
	}

	script := "#!/bin/sh\n" +
		"printf 'BEADS_DIR=%s\\n' \"$BEADS_DIR\"\n"
	binaryPath := writeExecutableScript(t, script)

	c := &Client{
		binary: binaryPath,
		Dir:    worktreeDir,
	}
	out, err := c.runWithEnv(context.Background(), []string{"print-env"}, nil)
	if err != nil {
		t.Fatalf("runWithEnv() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	got := make(map[string]string)
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			got[parts[0]] = parts[1]
		}
	}

	wantBeadsDir := filepath.Join(mainDir, ".beads")
	gotInfo, gotErr := os.Stat(got["BEADS_DIR"])
	if gotErr != nil {
		t.Fatalf("Stat(BEADS_DIR=%q): %v", got["BEADS_DIR"], gotErr)
	}
	wantInfo, wantErr := os.Stat(wantBeadsDir)
	if wantErr != nil {
		t.Fatalf("Stat(wantBeadsDir=%q): %v", wantBeadsDir, wantErr)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("BEADS_DIR = %q, want %q", got["BEADS_DIR"], wantBeadsDir)
	}
}

func TestClientRunWithEnv_PreservesExplicitBeadsDir(t *testing.T) {
	script := "#!/bin/sh\n" +
		"printf 'BEADS_DIR=%s\\n' \"$BEADS_DIR\"\n"
	binaryPath := writeExecutableScript(t, script)

	restore := restoreBeadProcutilFns(t)
	subprocessEnvFn = func() []string {
		return []string{"BEADS_DIR=/tmp/explicit-beads"}
	}

	c := &Client{binary: binaryPath}
	out, err := c.runWithEnv(context.Background(), []string{"print-env"}, nil)
	restore()
	if err != nil {
		t.Fatalf("runWithEnv() error = %v", err)
	}

	got := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(out), "BEADS_DIR="))
	if got != "/tmp/explicit-beads" {
		t.Fatalf("BEADS_DIR = %q, want %q", got, "/tmp/explicit-beads")
	}
}

func runInDir(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(out))
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
	restore := restoreBeadProcutilFns(t)
	waitForProcessCapacityFn = func(ctx context.Context, maxWait time.Duration) error {
		waitCalled = true
		if maxWait <= 0 {
			t.Fatalf("expected positive maxWait, got %v", maxWait)
		}
		return nil
	}

	var reapCalled bool
	reapProcessTreeFn = func(cmd *exec.Cmd) {
		reapCalled = true
	}

	c := &Client{Dir: repoDir}
	got, err := c.repoBaseName(context.Background())
	restore()
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

func TestClientRepoBaseNameRequiresContext(t *testing.T) {
	repoDir := t.TempDir()
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	c := &Client{Dir: repoDir}
	if _, err := c.repoBaseName(nil); !errors.Is(err, errContextRequired) {
		t.Fatalf("repoBaseName(nil) error = %v, want %v", err, errContextRequired)
	}
}

func TestRestoreBeadProcutilFns(t *testing.T) {
	origWait := waitForProcessCapacityFn
	origKill := killDescendantsOnCancelFn
	origReap := reapProcessTreeFn
	origEnv := subprocessEnvFn
	origResolve := resolveBeadsDirFn

	restore := restoreBeadProcutilFns(t)

	getPtr := func(fn any) uintptr {
		if fn == nil {
			return 0
		}
		return reflect.ValueOf(fn).Pointer()
	}

	customWait := func(ctx context.Context, maxWait time.Duration) error { return fmt.Errorf("change") }
	customKill := func(ctx context.Context, cmd *exec.Cmd) {}
	customReap := func(cmd *exec.Cmd) {}
	customEnv := func() []string { return []string{"FOO=bar"} }
	customResolve := func(ctx context.Context, dir string) string { return "/tmp" }

	waitForProcessCapacityFn = customWait
	killDescendantsOnCancelFn = customKill
	reapProcessTreeFn = customReap
	subprocessEnvFn = customEnv
	resolveBeadsDirFn = customResolve

	if getPtr(waitForProcessCapacityFn) == getPtr(origWait) {
		t.Fatal("wait hook was not replaced")
	}
	if getPtr(killDescendantsOnCancelFn) == getPtr(origKill) {
		t.Fatal("kill hook was not replaced")
	}
	if getPtr(reapProcessTreeFn) == getPtr(origReap) {
		t.Fatal("reap hook was not replaced")
	}
	if getPtr(subprocessEnvFn) == getPtr(origEnv) {
		t.Fatal("env hook was not replaced")
	}
	if getPtr(resolveBeadsDirFn) == getPtr(origResolve) {
		t.Fatal("resolve hook was not replaced")
	}

	restore()

	if getPtr(waitForProcessCapacityFn) != getPtr(origWait) {
		t.Fatal("wait hook was not restored")
	}
	if getPtr(killDescendantsOnCancelFn) != getPtr(origKill) {
		t.Fatal("kill hook was not restored")
	}
	if getPtr(reapProcessTreeFn) != getPtr(origReap) {
		t.Fatal("reap hook was not restored")
	}
	if getPtr(subprocessEnvFn) != getPtr(origEnv) {
		t.Fatal("env hook was not restored")
	}
	if getPtr(resolveBeadsDirFn) != getPtr(origResolve) {
		t.Fatal("resolve hook was not restored")
	}
}
