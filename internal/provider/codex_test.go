package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// newTestBinary creates a temporary executable file with the given bash script.
// It delegates to the shared testCreateBinaryWithETXTBSYProtection helper
// which ensures the file is properly synced before returning to avoid ETXTBSY errors
// under parallel test execution.
func newTestBinary(t *testing.T, bashScript string) string {
	return testCreateBinaryWithETXTBSYProtection(t, bashScript)
}

// newShellProvider creates a CodexProvider that executes shell commands via /bin/sh -c,
// avoiding temporary executable files and ETXTBSY race conditions.
// It takes a bash script string and returns a provider that executes it via shell.
func newShellProvider(bashScript string, tierMap map[string]string) *CodexProvider {
	// Use /bin/sh -c to execute bash script inline, avoiding temp file races (ETXTBSY)
	// under high parallel test load. The "--" terminates flag parsing.
	return NewCodexProvider("/bin/sh", []string{"-c", bashScript, "--"}, tierMap)
}

// TestCodexProviderStructExists verifies that CodexProvider struct exists
// and can be instantiated.
// Expected failure: CodexProvider struct does not exist yet
func TestCodexProviderStructExists(t *testing.T) {
	t.Parallel()
	var cp *CodexProvider
	if cp != nil {
		t.Error("nil CodexProvider should be nil")
	}
}

// TestCodexProviderImplementsProviderInterface verifies that CodexProvider
// satisfies the Provider interface via compile-time check.
// Expected failure: CodexProvider struct does not exist yet
func TestCodexProviderImplementsProviderInterface(t *testing.T) {
	t.Parallel()
	var _ Provider = (*CodexProvider)(nil)
}

// TestCodexProviderHasBinaryPathField verifies that CodexProvider has a
// binaryPath field for storing the path to the codex CLI binary.
// Expected failure: CodexProvider struct and binaryPath field do not exist yet
func TestCodexProviderHasBinaryPathField(t *testing.T) {
	t.Parallel()
	cp := &CodexProvider{
		binaryPath: "/usr/local/bin/codex",
	}

	if cp.binaryPath != "/usr/local/bin/codex" {
		t.Errorf("binaryPath = %q, want %q", cp.binaryPath, "/usr/local/bin/codex")
	}
}

// TestCodexProviderHasFlagsField verifies that CodexProvider has a flags field
// for storing CLI flags to pass to the codex binary.
// Expected failure: CodexProvider struct and flags field do not exist yet
func TestCodexProviderHasFlagsField(t *testing.T) {
	t.Parallel()
	flags := []string{"--verbose", "--no-color"}
	cp := &CodexProvider{
		flags: flags,
	}

	if len(cp.flags) != 2 {
		t.Errorf("len(flags) = %d, want 2", len(cp.flags))
	}
	if cp.flags[0] != "--verbose" {
		t.Errorf("flags[0] = %q, want %q", cp.flags[0], "--verbose")
	}
}

// TestCodexProviderHasTierToModelMap verifies that CodexProvider has a
// tierToModel map field for mapping abstract tiers to Codex-specific model names.
// Expected failure: CodexProvider struct and tierToModel field do not exist yet
func TestCodexProviderHasTierToModelMap(t *testing.T) {
	t.Parallel()
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := &CodexProvider{
		tierToModel: tierMap,
	}

	if cp.tierToModel == nil {
		t.Error("CodexProvider.tierToModel should not be nil after assignment")
	}

	if cp.tierToModel[TierHigh] != "o3" {
		t.Errorf("tierToModel[TierHigh] = %q, want %q", cp.tierToModel[TierHigh], "o3")
	}
	if cp.tierToModel[TierMedium] != "gpt-4o" {
		t.Errorf("tierToModel[TierMedium] = %q, want %q", cp.tierToModel[TierMedium], "gpt-4o")
	}
	if cp.tierToModel[TierLow] != "gpt-4o-mini" {
		t.Errorf("tierToModel[TierLow] = %q, want %q", cp.tierToModel[TierLow], "gpt-4o-mini")
	}
}

// TestNewCodexProviderConstructor verifies that NewCodexProvider constructor
// creates a CodexProvider with all required fields set correctly.
// Expected failure: NewCodexProvider() function does not exist yet
func TestNewCodexProviderConstructor(t *testing.T) {
	t.Parallel()
	binaryPath := "/usr/local/bin/codex"
	flags := []string{"--no-color"}
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := NewCodexProvider(binaryPath, flags, tierMap)

	if cp == nil {
		t.Fatal("NewCodexProvider() returned nil")
	}

	if cp.binaryPath != binaryPath {
		t.Errorf("binaryPath = %q, want %q", cp.binaryPath, binaryPath)
	}

	if len(cp.flags) != len(flags) || cp.flags[0] != flags[0] {
		t.Errorf("flags = %v, want %v", cp.flags, flags)
	}

	if cp.tierToModel[TierHigh] != "o3" {
		t.Errorf("tierToModel[TierHigh] = %q, want %q", cp.tierToModel[TierHigh], "o3")
	}
}

// TestCodexProviderNameMethod verifies that CodexProvider implements
// Name() method returning "codex".
// Expected failure: CodexProvider struct and Name() method do not exist yet
func TestCodexProviderNameMethod(t *testing.T) {
	t.Parallel()
	cp := &CodexProvider{}

	name := cp.Name()

	if name != "codex" {
		t.Errorf("Name() = %q, want %q", name, "codex")
	}
}

// TestCodexProviderRunCapturesStdout verifies that Run() captures the
// standard output from the codex CLI invocation.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunCapturesStdout(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `echo "Test output line 1"
echo "Test output line 2"
exit 0`)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !strings.Contains(result.Output, "Test output line 1") {
		t.Errorf("Run() output missing expected stdout, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Test output line 2") {
		t.Errorf("Run() output missing expected stdout line 2, got: %s", result.Output)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		t.Errorf("Run() stderr should be empty for stdout-only command, got: %q", result.Stderr)
	}
}

func TestCodexRunOnceUsesKillDescendantsOnCancel(t *testing.T) {
	t.Parallel()
	prompt := "test prompt"
	const model = "gpt-5.3-codex"
	script := strings.Join([]string{
		`printf '{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}\n'`,
		`printf '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1,"total_cost_usd":0}}\n'`,
	}, "\n") + "\n"
	mockBinary := testCreateBinaryWithETXTBSYProtection(t, script)
	cp := NewCodexProvider(mockBinary, []string{}, map[string]string{TierLow: model})
	args := cp.buildCommandArgsForTier(model, TierLow, true)
	env, home, err := prepareCodexEnv()
	if err != nil {
		t.Fatalf("prepareCodexEnv() error = %v", err)
	}
	ctx := context.Background()
	var killCalled bool
	oldKill := codexKillDescendantsOnCancelFn
	t.Cleanup(func() { codexKillDescendantsOnCancelFn = oldKill })
	codexKillDescendantsOnCancelFn = func(ctx context.Context, cmd *exec.Cmd) {
		killCalled = true
	}
	result, err := cp.runOnce(ctx, prompt, model, args, env, home)
	if err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if result == nil {
		t.Fatal("runOnce() returned nil result")
	}
	if !killCalled {
		t.Fatal("runOnce() did not call KillDescendantsOnCancel")
	}
}

func TestCodexProviderRunDoesNotMixStderrIntoOutput(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `echo "stdout line"
echo "stderr line" >&2
exit 1`)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !strings.Contains(result.Output, "stdout line") {
		t.Errorf("Run() output missing stdout content, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "stderr line") {
		t.Errorf("Run() output should not include stderr content, got: %s", result.Output)
	}
	if !strings.Contains(result.Stderr, "stderr line") {
		t.Errorf("Run() stderr missing expected content, got: %q", result.Stderr)
	}
}

// TestCodexProviderRunCapturesStderr verifies that Run() captures the
// standard error output from the codex CLI invocation.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunCapturesStderr(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `echo "Error message" >&2
exit 1`)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	// Run should not return an error for non-zero exit codes
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !strings.Contains(result.Stderr, "Error message") {
		t.Errorf("Run() stderr missing expected content, got: %q", result.Stderr)
	}
	if strings.TrimSpace(result.Output) != "" {
		t.Errorf("Run() output should be empty for stderr-only command, got output=%q", result.Output)
	}

	if result.ExitCode != 1 {
		t.Errorf("Run() ExitCode = %d, want 1", result.ExitCode)
	}

	if result.Success {
		t.Error("Run() Success should be false for non-zero exit code")
	}
}

func TestCodexProviderRun_CreatesMissingCODEXHOME(t *testing.T) {
	tempDir := t.TempDir()
	missingHome := filepath.Join(tempDir, "codex-home-missing")
	t.Setenv("CODEX_HOME", missingHome)
	t.Setenv("EXPECTED_BAD_CODEX_HOME", missingHome)

	mockBinary := newTestBinary(t, `echo "CODEX_HOME=$CODEX_HOME"
if [ ! -d "$CODEX_HOME" ]; then
  echo "missing CODEX_HOME directory: $CODEX_HOME" >&2
  exit 9
fi
if [ "$CODEX_HOME" = "$EXPECTED_BAD_CODEX_HOME" ]; then
  echo "CODEX_HOME was not rewritten from temp path" >&2
  exit 8
fi
echo "ok"
exit 0`)

	cp := NewCodexProvider(mockBinary, []string{}, map[string]string{TierLow: "gpt-4o-mini"})
	result, err := cp.Run(context.Background(), "test", TierLow)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("Run() should succeed after creating CODEX_HOME, got result=%+v", result)
	}
	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "CODEX_HOME=") {
		t.Fatalf("expected output to start with CODEX_HOME=..., got: %q", result.Output)
	}
	actualHome := strings.TrimPrefix(lines[0], "CODEX_HOME=")
	if actualHome == missingHome {
		t.Fatalf("expected CODEX_HOME to be rewritten away from temp path %q", missingHome)
	}
	if _, err := os.Stat(actualHome); err != nil {
		t.Fatalf("expected rewritten CODEX_HOME dir to exist after run: %v", err)
	}
}

func TestPrepareCodexEnv_RemovesCODEXCI(t *testing.T) {
	t.Setenv("CODEX_CI", "1")
	t.Setenv("CODEX_HOME", "")

	env, _, err := prepareCodexEnv()
	if err != nil {
		t.Fatalf("prepareCodexEnv() error = %v", err)
	}

	for _, kv := range env {
		if strings.HasPrefix(kv, "CODEX_CI=") {
			t.Fatalf("prepareCodexEnv() should remove CODEX_CI, found %q", kv)
		}
	}
}

func TestRemoveEnvKey_DropsOnlyMatchingKey(t *testing.T) {
	t.Parallel()
	env := []string{
		"PATH=/usr/bin",
		"CODEX_CI=1",
		"CODEX_HOME=/tmp/home",
		"OTHER=ok",
	}

	got := removeEnvKey(env, "CODEX_CI")

	if len(got) != 3 {
		t.Fatalf("removeEnvKey() len = %d, want 3", len(got))
	}
	for _, kv := range got {
		if strings.HasPrefix(kv, "CODEX_CI=") {
			t.Fatalf("removeEnvKey() retained CODEX_CI entry: %q", kv)
		}
	}
	if !containsEnvKV(got, "CODEX_HOME=/tmp/home") {
		t.Fatal("removeEnvKey() unexpectedly removed CODEX_HOME")
	}
}

func containsEnvKV(env []string, target string) bool {
	for _, kv := range env {
		if kv == target {
			return true
		}
	}
	return false
}

func TestCodexProviderRun_RetriesTransientFailureOnce(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	counterFile := filepath.Join(tempDir, "attempt-counter")
	mockBinary := newTestBinary(t, `COUNT=0
if [ -f "`+counterFile+`" ]; then
  COUNT=$(cat "`+counterFile+`")
fi
COUNT=$((COUNT+1))
echo "$COUNT" > "`+counterFile+`"
if [ "$COUNT" -eq 1 ]; then
  echo "stream disconnected during request" >&2
  exit 1
fi
echo "ok after retry"
exit 0`)

	cp := NewCodexProvider(mockBinary, []string{}, map[string]string{TierLow: "gpt-4o-mini"})
	cp.sleepFn = func(context.Context, time.Duration) error { return nil }
	result, err := cp.Run(context.Background(), "test", TierLow)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("Run() should succeed after transient retry, got result=%+v", result)
	}
	if !strings.Contains(result.Output, "ok after retry") {
		t.Fatalf("expected output from retry success, got: %q", result.Output)
	}
	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("failed to read attempt counter: %v", err)
	}
	if strings.TrimSpace(string(data)) != "2" {
		t.Fatalf("expected 2 attempts, got %q", strings.TrimSpace(string(data)))
	}
}

func TestCodexProvider_runWithRetry_RetriesRetryableStartError(t *testing.T) {
	t.Parallel()
	cp := NewCodexProvider("/bin/false", nil, map[string]string{})

	attempts := 0
	sleepCalls := 0
	cp.sleepFn = func(context.Context, time.Duration) error {
		sleepCalls++
		return nil
	}

	expected := &Result{Success: true, Output: "ok"}
	result, err := cp.runWithRetry(context.Background(), func() (*Result, error) {
		attempts++
		if attempts == 1 {
			return nil, fmt.Errorf("failed to start codex command: %w", &os.PathError{Op: "fork/exec", Path: "/usr/bin/codex", Err: syscall.EAGAIN})
		}
		return expected, nil
	})

	if err != nil {
		t.Fatalf("runWithRetry() err = %v, want nil", err)
	}
	if result != expected {
		t.Fatalf("runWithRetry() returned unexpected result: %+v", result)
	}
	if attempts != 2 {
		t.Fatalf("runWithRetry() attempts = %d, want 2", attempts)
	}
	if sleepCalls != 1 {
		t.Fatalf("runWithRetry() sleep calls = %d, want 1", sleepCalls)
	}
}

func TestCodexProvider_runWithRetry_DoesNotRetryNonRetryableStartError(t *testing.T) {
	t.Parallel()
	cp := NewCodexProvider("/bin/false", nil, map[string]string{})

	attempts := 0
	sleepCalls := 0
	cp.sleepFn = func(context.Context, time.Duration) error {
		sleepCalls++
		return nil
	}

	startErr := errors.New("failed to start codex command: exec format error")
	result, err := cp.runWithRetry(context.Background(), func() (*Result, error) {
		attempts++
		return nil, startErr
	})

	if !errors.Is(err, startErr) {
		t.Fatalf("runWithRetry() err = %v, want %v", err, startErr)
	}
	if result != nil {
		t.Fatalf("runWithRetry() result = %+v, want nil", result)
	}
	if attempts != 1 {
		t.Fatalf("runWithRetry() attempts = %d, want 1", attempts)
	}
	if sleepCalls != 0 {
		t.Fatalf("runWithRetry() sleep calls = %d, want 0", sleepCalls)
	}
}

func TestCodexProviderRun_FailureIncludesDiagnostics(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `echo "fatal transport issue" >&2
exit 4`)

	cp := NewCodexProvider(mockBinary, []string{"--dangerously-bypass-approvals-and-sandbox"}, map[string]string{TierLow: "gpt-4o-mini"})
	result, err := cp.Run(context.Background(), "test", TierLow)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result == nil || result.Success {
		t.Fatalf("Run() should fail, got result=%+v", result)
	}
	if !strings.Contains(result.Diagnostics, "codex_args=exec") {
		t.Fatalf("Diagnostics missing args, got: %q", result.Diagnostics)
	}
	if !strings.Contains(result.Diagnostics, "stderr_head=") || !strings.Contains(result.Diagnostics, "stderr_tail=") {
		t.Fatalf("Diagnostics missing stderr head/tail, got: %q", result.Diagnostics)
	}
}

func TestClassifyCodexFailure(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		exitCode int
		stdout   string
		stderr   string
		want     string
	}{
		{
			name:     "success has no category",
			exitCode: 0,
			want:     FailureCategoryNone,
		},
		{
			name:     "transport disconnect",
			exitCode: 1,
			stderr:   "ERROR: stream disconnected before completion",
			want:     FailureCategoryTransportDisconnect,
		},
		{
			name:     "startup error",
			exitCode: 1,
			stderr:   "ERROR: failed to start codex command: exec format error",
			want:     FailureCategoryStartupError,
		},
		{
			name:     "dns resolution failure is transport",
			exitCode: 1,
			stderr:   "ERROR: could not resolve host: api.openai.com",
			want:     FailureCategoryTransportDisconnect,
		},
		{
			name:     "rate limited",
			exitCode: 1,
			stderr:   "429 too many requests",
			want:     FailureCategoryRateLimited,
		},
		{
			name:     "auth",
			exitCode: 1,
			stderr:   "invalid api key",
			want:     FailureCategoryAuth,
		},
		{
			name:     "other",
			exitCode: 2,
			stderr:   "some unknown failure",
			want:     FailureCategoryOther,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := classifyCodexFailure(tc.exitCode, tc.stdout, tc.stderr)
			if got != tc.want {
				t.Fatalf("classifyCodexFailure(%d, %q, %q) = %q, want %q", tc.exitCode, tc.stdout, tc.stderr, got, tc.want)
			}
		})
	}
}

// TestCodexProviderRunReturnsResultWithDuration verifies that Run() populates
// the Result.Duration field with the actual execution time.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunReturnsResultWithDuration(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `sleep 0.1
echo "done"
exit 0`)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if result.Duration < 50*time.Millisecond {
		t.Errorf("Run() Duration = %v, expected at least 50ms", result.Duration)
	}

	if result.Duration > 5*time.Second {
		t.Errorf("Run() Duration = %v, unexpectedly long", result.Duration)
	}
}

// TestCodexProviderRunSetsSuccessBasedOnExitCode verifies that Run() sets
// Result.Success to true when exit code is 0, false otherwise.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunSetsSuccessBasedOnExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		exitCode    int
		wantSuccess bool
	}{
		{
			name:        "exit code 0 means success",
			exitCode:    0,
			wantSuccess: true,
		},
		{
			name:        "exit code 1 means failure",
			exitCode:    1,
			wantSuccess: false,
		},
		{
			name:        "exit code 2 means failure",
			exitCode:    2,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBinary := newTestBinary(t, "exit "+string(rune('0'+tt.exitCode)))

			tierMap := map[string]string{TierLow: "gpt-4o-mini"}
			cp := NewCodexProvider(mockBinary, []string{}, tierMap)

			ctx := context.Background()
			result, err := cp.Run(ctx, "test", TierLow)

			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("Run() Success = %v, want %v for exit code %d",
					result.Success, tt.wantSuccess, tt.exitCode)
			}

			if result.ExitCode != tt.exitCode {
				t.Errorf("Run() ExitCode = %d, want %d", result.ExitCode, tt.exitCode)
			}
		})
	}
}

// TestCodexProviderRunPopulatesModelInResult verifies that Run() sets the
// Result.Model field to the resolved model name from the tier mapping.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunPopulatesModelInResult(t *testing.T) {
	t.Parallel()

	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}
	// Use /bin/sh directly to avoid temp-file executable races (ETXTBSY)
	// under high parallel test load.
	cp := NewCodexProvider("/bin/sh", []string{"-c", "echo done; exit 0", "--"}, tierMap)

	tests := []struct {
		tier      string
		wantModel string
	}{
		{TierHigh, "o3"},
		{TierMedium, "gpt-4o"},
		{TierLow, "gpt-4o-mini"},
	}

	for _, tt := range tests {
		t.Run("tier_"+tt.tier, func(t *testing.T) {
			ctx := context.Background()
			result, err := cp.Run(ctx, "test", tt.tier)

			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}

			if result.Model != tt.wantModel {
				t.Errorf("Run() Model = %q, want %q for tier %s",
					result.Model, tt.wantModel, tt.tier)
			}
		})
	}
}

// TestCodexProviderStreamRunStreamsOutput verifies that StreamRun() writes
// output to the provided io.Writer as it's produced by the codex CLI.
// Expected failure: CodexProvider StreamRun() method does not exist yet
func TestCodexProviderStreamRunStreamsOutput(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.agentMessage.delta","delta":{"text":"Line 1\n"}}'
sleep 0.05
echo '{"type":"item.agentMessage.delta","delta":{"text":"Line 2\n"}}'
sleep 0.05
echo '{"type":"item.agentMessage.delta","delta":{"text":"Line 3\n"}}'
echo '{"type":"item.completed","item":{"type":"agent_message","text":"Line 1\nLine 2\nLine 3"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5}}'
exit 0`)

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	result, err := cp.StreamRun(ctx, "test prompt", TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "prompt length:") || !strings.Contains(outputStr, "cmd:") {
		t.Errorf("StreamRun() should include invocation metadata, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Line 1") {
		t.Errorf("StreamRun() output missing 'Line 1', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Line 2") {
		t.Errorf("StreamRun() output missing 'Line 2', got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Line 3") {
		t.Errorf("StreamRun() output missing 'Line 3', got: %s", outputStr)
	}
}

func TestCodexProviderStreamRunUsesAutoColor(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `echo "ARGS: $@"
exit 0`)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierMedium: "gpt-4o"})

	var output bytes.Buffer
	result, err := cp.StreamRun(context.Background(), "test", TierMedium, &output, nil, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	got := result.Output + output.String()
	if !strings.Contains(got, "--color auto") {
		t.Errorf("StreamRun() should use '--color auto', got: %s", got)
	}
	if strings.Contains(got, "--color never") {
		t.Errorf("StreamRun() should not force '--color never', got: %s", got)
	}
}

// TestCodexProviderStreamRunEventHandlerCalledWithJSON verifies that StreamRun()
// invokes EventHandler when a non-nil handler is provided and the binary emits JSONL.
// Expected failure: CodexProvider StreamRun() does not add --json flag or call handler yet
func TestCodexProviderStreamRunEventHandlerCalledWithJSON(t *testing.T) {
	t.Parallel()
	// Mock binary that emits a JSONL event when --json flag is present
	mockBinary := newTestBinary(t, `if [[ "$*" == *"--json"* ]]; then
    echo '{"type":"thread.started","data":{"thread_id":"t-123"}}'
else
    echo 'plain text output'
fi
exit 0`)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	// EventHandler SHOULD be called when non-nil and --json is active
	handlerCalled := false
	handler := func(line []byte) {
		handlerCalled = true
	}

	result, err := cp.StreamRun(ctx, "test", TierLow, &output, handler, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	// EventHandler SHOULD be called when non-nil (after implementation)
	if !handlerCalled {
		t.Error("StreamRun() with non-nil EventHandler should invoke handler for JSONL events")
	}
}

// TestCodexProviderStreamRunToolCallHandlerCalledForToolEvents verifies that StreamRun()
// invokes ToolCallHandler when tool-related events are emitted in JSONL format.
// Expected failure: CodexProvider StreamRun() does not parse tool events or call handler yet
func TestCodexProviderStreamRunToolCallHandlerCalledForToolEvents(t *testing.T) {
	t.Parallel()
	// Mock binary that emits a command_execution event when --json is present
	mockBinary := newTestBinary(t, `if [[ "$*" == *"--json"* ]]; then
    echo '{"type":"item.started","item":{"type":"command_execution","command":"go test"}}'
else
    echo 'plain text output'
fi
exit 0`)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer

	// ToolCallHandler SHOULD be called for tool-related item.started events
	handlerCalled := false
	var receivedEvent ToolEvent
	toolHandler := func(event ToolEvent) {
		handlerCalled = true
		receivedEvent = event
	}

	result, err := cp.StreamRun(ctx, "test", TierLow, &output, func(line []byte) {}, toolHandler)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	// ToolCallHandler SHOULD be called when non-nil and tool events are present
	if !handlerCalled {
		t.Error("StreamRun() with non-nil ToolCallHandler should invoke handler for tool events")
	}

	// Verify the tool event has correct fields
	if handlerCalled && receivedEvent.ToolName != "Bash" {
		t.Errorf("ToolEvent.ToolName = %q, want %q for command_execution", receivedEvent.ToolName, "Bash")
	}
}

// TestCodexProviderStreamRunReturnsResultWithMetadata verifies that StreamRun()
// returns a Result with all metadata fields populated correctly.
// Expected failure: CodexProvider StreamRun() method does not exist yet
func TestCodexProviderStreamRunReturnsResultWithMetadata(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.completed","item":{"type":"agent_message","text":"success"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":5}}'
exit 0`)

	tierMap := map[string]string{TierMedium: "gpt-4o"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	var output bytes.Buffer
	result, err := cp.StreamRun(ctx, "test", TierMedium, &output, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	if !result.Success {
		t.Error("StreamRun() Success = false, want true for exit code 0")
	}

	if result.ExitCode != 0 {
		t.Errorf("StreamRun() ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Model != "gpt-4o" {
		t.Errorf("StreamRun() Model = %q, want %q", result.Model, "gpt-4o")
	}

	if result.Duration <= 0 {
		t.Errorf("StreamRun() Duration = %v, want > 0", result.Duration)
	}

	if !strings.Contains(result.Output, "success") {
		t.Errorf("StreamRun() Output missing expected content, got: %s", result.Output)
	}
}

// TestCodexProviderIsUsageLimitErrorDetectsOpenAIErrors verifies that
// IsUsageLimitError() detects OpenAI/Codex-specific usage limit patterns.
// Expected failure: CodexProvider IsUsageLimitError() method does not exist yet
func TestCodexProviderIsUsageLimitErrorDetectsOpenAIErrors(t *testing.T) {
	t.Parallel()
	cp := &CodexProvider{}

	tests := []struct {
		name     string
		result   *Result
		err      error
		expected bool
	}{
		{
			name: "detects quota exceeded",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Error: quota exceeded for this model",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "detects rate limit",
			result: &Result{
				Success:  false,
				ExitCode: 429,
				Output:   "Rate limit exceeded. Please try again later.",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "detects usage limit",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "usage limit reached",
			},
			err:      nil,
			expected: true,
		},
		{
			name: "does not detect generic errors",
			result: &Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Error: invalid prompt format",
			},
			err:      nil,
			expected: false,
		},
		{
			name:     "nil result returns false",
			result:   nil,
			err:      nil,
			expected: false,
		},
		{
			name: "successful result returns false",
			result: &Result{
				Success:  true,
				ExitCode: 0,
				Output:   "completed successfully",
			},
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cp.IsUsageLimitError(tt.result, tt.err)
			if got != tt.expected {
				t.Errorf("IsUsageLimitError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestCodexProviderModelForTierReturnsCorrectModel verifies that ModelForTier()
// maps tier constants to the configured Codex model names.
// Expected failure: CodexProvider ModelForTier() method does not exist yet
func TestCodexProviderModelForTierReturnsCorrectModel(t *testing.T) {
	t.Parallel()
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}

	cp := &CodexProvider{
		tierToModel: tierMap,
	}

	tests := []struct {
		tier      string
		wantModel string
	}{
		{TierHigh, "o3"},
		{TierMedium, "gpt-4o"},
		{TierLow, "gpt-4o-mini"},
	}

	for _, tt := range tests {
		t.Run("tier_"+tt.tier, func(t *testing.T) {
			t.Parallel()
			got := cp.ModelForTier(tt.tier)
			if got != tt.wantModel {
				t.Errorf("ModelForTier(%q) = %q, want %q", tt.tier, got, tt.wantModel)
			}
		})
	}
}

// TestCodexProviderRunWithContextCancellation verifies that Run() respects
// context cancellation and stops execution when the context is cancelled.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunWithContextCancellation(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Fatal("skipping context cancellation test in short mode")
	}

	mockBinary := newTestBinary(t, `sleep 1
echo 'done'
exit 0`)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := cp.Run(ctx, "test", TierLow)

	// Should return an error due to context timeout
	if err == nil {
		t.Error("Run() error = nil, want context deadline exceeded error")
	}

	if err != nil && !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Run() error = %q, want context cancellation error", err.Error())
	}

	// Result may be nil or partial
	_ = result
}

// TestCodexProviderStreamRunWithContextCancellationJSONMode verifies that
// StreamRun() in JSON mode (non-nil handler) returns a context cancellation
// error when the invocation context expires.
func TestCodexProviderStreamRunWithContextCancellationJSONMode(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Fatal("skipping context cancellation test in short mode")
	}

	mockBinary := newTestBinary(t, `sleep 1
echo '{"type":"turn.completed"}'
exit 0`)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var output bytes.Buffer
	_, err := cp.StreamRun(ctx, "test", TierLow, &output, func([]byte) {}, nil)
	if err == nil {
		t.Fatal("StreamRun() error = nil, want context cancellation error")
	}
	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("StreamRun() error = %q, want context cancellation/deadline error", err.Error())
	}
}

func TestCodexProviderRun_PropagatesUsageToResult(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}'
echo '{"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":3210,"cached_input_tokens":210,"output_tokens":987,"total_cost_usd":0.051}}}'
exit 0`)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierMedium: "gpt-5.3-codex"})
	ctx := context.Background()
	result, err := cp.Run(ctx, "prompt", TierMedium)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Output != "done" {
		t.Errorf("Output = %q, want %q", result.Output, "done")
	}
	if result.InputTokens != 3210 {
		t.Errorf("InputTokens = %d, want 3210", result.InputTokens)
	}
	if result.CachedInputTokens != 210 {
		t.Errorf("CachedInputTokens = %d, want 210", result.CachedInputTokens)
	}
	if result.OutputTokens != 987 {
		t.Errorf("OutputTokens = %d, want 987", result.OutputTokens)
	}
	if result.CostUSD != 0.051 {
		t.Errorf("CostUSD = %v, want 0.051", result.CostUSD)
	}
}

func TestCodexProviderRun_PropagatesTurnCompletedUsageToResult(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.completed","item":{"type":"agent_message","text":"done from turn"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":111,"cached_input_tokens":22,"output_tokens":333,"total_cost_usd":0.004}}'
exit 0`)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierMedium: "gpt-5.3-codex"})
	result, err := cp.Run(context.Background(), "prompt", TierMedium)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Output != "done from turn" {
		t.Errorf("Output = %q, want %q", result.Output, "done from turn")
	}
	if result.InputTokens != 111 {
		t.Errorf("InputTokens = %d, want 111", result.InputTokens)
	}
	if result.CachedInputTokens != 22 {
		t.Errorf("CachedInputTokens = %d, want 22", result.CachedInputTokens)
	}
	if result.OutputTokens != 333 {
		t.Errorf("OutputTokens = %d, want 333", result.OutputTokens)
	}
	if result.CostUSD != 0.004 {
		t.Errorf("CostUSD = %v, want 0.004", result.CostUSD)
	}
}

func TestCodexProviderStreamRun_PropagatesUsageToResult(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.completed","item":{"type":"agent_message","text":"done"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":1234,"cached_input_tokens":567,"output_tokens":890,"total_cost_usd":0.042}}'
exit 0`)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierMedium: "gpt-5.3-codex"})
	ctx := context.Background()

	// Non-nil handler enables --json mode and usage extraction.
	result, err := cp.StreamRun(ctx, "prompt", TierMedium, nil, func([]byte) {}, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}
	if result.InputTokens != 1234 {
		t.Errorf("InputTokens = %d, want 1234", result.InputTokens)
	}
	if result.CachedInputTokens != 567 {
		t.Errorf("CachedInputTokens = %d, want 567", result.CachedInputTokens)
	}
	if result.OutputTokens != 890 {
		t.Errorf("OutputTokens = %d, want 890", result.OutputTokens)
	}
	if result.CostUSD != 0.042 {
		t.Errorf("CostUSD = %v, want 0.042", result.CostUSD)
	}
}

func TestCodexProviderStreamRun_FailureOutputExcludesStderr(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.completed","item":{"type":"agent_message","text":"stream ok"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
echo "stream error" >&2
exit 1`)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierMedium: "gpt-5.3-codex"})
	ctx := context.Background()

	result, err := cp.StreamRun(ctx, "prompt", TierMedium, nil, func([]byte) {}, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}
	if result.Success {
		t.Fatalf("StreamRun() Success = true, want false on failure")
	}
	if !strings.Contains(result.Output, "stream ok") {
		t.Errorf("StreamRun() output missing agent text, got: %q", result.Output)
	}
	if strings.Contains(result.Output, "stream error") {
		t.Errorf("StreamRun() output should not include stderr, got: %q", result.Output)
	}
	if !strings.Contains(result.Stderr, "stream error") {
		t.Errorf("StreamRun() stderr missing expected content, got: %q", result.Stderr)
	}
}

func TestCodexProviderStreamRunUsageLimitErrorIncludesUsage(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `cat > /dev/null
echo '{"type":"item.completed","item":{"type":"agent_message","text":"done"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":210,"output_tokens":80,"total_cost_usd":0.015}}'
echo '{"type":"turn.completed","status":"failed","error":{"type":"UsageLimitExceeded","message":"Limit hit"}}'
exit 0`)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierMedium: "gpt-5.3-codex"})
	ctx := context.Background()

	result, err := cp.StreamRun(ctx, "prompt", TierMedium, nil, func([]byte) {}, nil)
	if err == nil {
		t.Fatalf("StreamRun() err = nil, want UsageLimitError")
	}
	var usageErr *UsageLimitError
	if !errors.As(err, &usageErr) {
		t.Fatalf("StreamRun() err = %v, want UsageLimitError", err)
	}
	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}
	if result.InputTokens != 210 {
		t.Fatalf("InputTokens = %d, want 210", result.InputTokens)
	}
	if result.OutputTokens != 80 {
		t.Fatalf("OutputTokens = %d, want 80", result.OutputTokens)
	}
}

// Expected failure: codexStreamTransientRetryCategories constant does not exist yet and StreamRun does not retry transient failures.
func TestCodexProviderStreamRun_RetriesTransientFailuresAndPreservesJSONSemantics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                string
		firstAttemptStderr  string
		wantFailureCategory string
	}{
		{
			name:                "transport_disconnect_is_retried",
			firstAttemptStderr:  "ERROR: stream disconnected before completion",
			wantFailureCategory: FailureCategoryTransportDisconnect,
		},
		{
			name:                "rate_limited_is_retried",
			firstAttemptStderr:  "429 too many requests",
			wantFailureCategory: FailureCategoryRateLimited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tempDir := t.TempDir()
			counterFile := filepath.Join(tempDir, "attempt-counter")
			mockBinary := filepath.Join(tempDir, "codex")
			mockScript := fmt.Sprintf(`#!/bin/bash
COUNT=0
if [ -f %q ]; then
  COUNT=$(cat %q)
fi
COUNT=$((COUNT+1))
echo "$COUNT" > %q
cat > /dev/null
if [ "$COUNT" -eq 1 ]; then
  echo %q >&2
  exit 1
fi
echo '{"type":"item.completed","item":{"type":"agent_message","text":"done after retry"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":123,"cached_input_tokens":45,"output_tokens":67,"total_cost_usd":0.5}}'
exit 0
`, counterFile, counterFile, counterFile, tt.firstAttemptStderr)

			writeTestExecutable(t, mockBinary, mockScript)

			cp := NewCodexProvider(mockBinary, nil, map[string]string{TierMedium: "gpt-5.3-codex"})
			cp.sleepFn = func(context.Context, time.Duration) error { return nil }
			var events [][]byte
			result, err := cp.StreamRun(
				context.Background(),
				"prompt",
				TierMedium,
				nil,
				func(line []byte) { events = append(events, append([]byte(nil), line...)) },
				nil,
			)

			if err != nil {
				t.Fatalf("StreamRun() error = %v, want nil", err)
			}
			if result == nil {
				t.Fatal("StreamRun() returned nil result")
			}
			if !result.Success {
				t.Fatalf("StreamRun() Success = false, want true after transient retry. Result: %+v", result)
			}

			attempts := readAttemptCount(t, counterFile)
			if attempts != 2 {
				t.Fatalf("transient failure should be retried exactly once before success, attempts=%d", attempts)
			}
			if result.FailureCategory != FailureCategoryNone {
				t.Fatalf("FailureCategory = %q, want %q on retry success", result.FailureCategory, FailureCategoryNone)
			}
			if result.InputTokens != 123 || result.CachedInputTokens != 45 || result.OutputTokens != 67 || result.CostUSD != 0.5 {
				t.Fatalf("usage fields not preserved after retry: %+v", result)
			}
			if len(events) == 0 {
				t.Fatal("expected streamed JSON events after retry success, got none")
			}
			if !strings.Contains(result.Output, "done after retry") {
				t.Fatalf("output should contain second-attempt streamed assistant text, got %q", result.Output)
			}
		})
	}
}

// Expected failure: codexStreamTransientRetryTotalAttempts constant does not exist yet and StreamRun currently stops after the first failed attempt.
func TestCodexProviderStreamRun_BoundedTransientRetryBudgetAndFailureClassification(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	counterFile := filepath.Join(tempDir, "attempt-counter")
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := fmt.Sprintf(`#!/bin/bash
COUNT=0
if [ -f %q ]; then
  COUNT=$(cat %q)
fi
COUNT=$((COUNT+1))
echo "$COUNT" > %q
cat > /dev/null
echo "429 too many requests" >&2
exit 1
`, counterFile, counterFile, counterFile)
	writeTestExecutable(t, mockBinary, mockScript)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierMedium: "gpt-5.3-codex"})
	cp.sleepFn = func(context.Context, time.Duration) error { return nil }
	result, err := cp.StreamRun(context.Background(), "prompt", TierMedium, nil, func([]byte) {}, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	attempts := readAttemptCount(t, counterFile)
	if attempts < 2 || attempts > 3 {
		t.Fatalf("transient retry budget should be bounded to 2-3 total attempts, got %d", attempts)
	}
	if result.Success {
		t.Fatalf("StreamRun() Success = true, want false when all retry attempts fail")
	}
	if result.FailureCategory != FailureCategoryRateLimited {
		t.Fatalf("FailureCategory = %q, want %q after retry exhaustion", result.FailureCategory, FailureCategoryRateLimited)
	}
	if result.ExitCode != 1 {
		t.Fatalf("ExitCode = %d, want 1", result.ExitCode)
	}
}

// TestCodexProvider_SleepFnDefaultsToSleepWithContext verifies that NewCodexProvider
// sets sleepFn to a non-nil function (procutil.SleepWithContext) by default.
func TestCodexProvider_SleepFnDefaultsToSleepWithContext(t *testing.T) {
	t.Parallel()
	cp := NewCodexProvider("/bin/codex", nil, nil)
	if cp.sleepFn == nil {
		t.Error("NewCodexProvider() sleepFn should default to procutil.SleepWithContext, got nil")
	}
}

// TestCodexProvider_SleepFnCalledOnRetryInRun verifies that Run() uses p.sleepFn
// for retry backoff instead of calling procutil.SleepWithContext directly, allowing tests
// to inject an instant-return stub.
func TestCodexProvider_SleepFnCalledOnRetryInRun(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	counterFile := filepath.Join(tempDir, "attempt-counter")
	mockBinary := filepath.Join(tempDir, "codex")
	// First attempt: transient failure. Second attempt: success.
	mockScript := fmt.Sprintf(`#!/bin/bash
COUNT=0
if [ -f %q ]; then COUNT=$(cat %q); fi
COUNT=$((COUNT+1))
echo "$COUNT" > %q
cat > /dev/null
if [ "$COUNT" -eq 1 ]; then
  echo "stream disconnected" >&2
  exit 1
fi
echo "ok"
exit 0
`, counterFile, counterFile, counterFile)
	writeTestExecutable(t, mockBinary, mockScript)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierLow: "gpt-4o-mini"})

	sleepCalled := false
	cp.sleepFn = func(ctx context.Context, d time.Duration) error {
		sleepCalled = true
		return nil
	}

	result, err := cp.Run(context.Background(), "test", TierLow)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("Run() should succeed after retry, got %+v", result)
	}
	if !sleepCalled {
		t.Error("Run() should have called sleepFn during retry backoff")
	}
}

// TestCodexProvider_SleepFnCalledOnRetryInStreamRun verifies that StreamRun() uses
// p.sleepFn for retry backoff, allowing tests to inject an instant-return stub.
func TestCodexProvider_SleepFnCalledOnRetryInStreamRun(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	counterFile := filepath.Join(tempDir, "attempt-counter")
	mockBinary := filepath.Join(tempDir, "codex")
	mockScript := fmt.Sprintf(`#!/bin/bash
COUNT=0
if [ -f %q ]; then COUNT=$(cat %q); fi
COUNT=$((COUNT+1))
echo "$COUNT" > %q
cat > /dev/null
if [ "$COUNT" -eq 1 ]; then
  echo "stream disconnected" >&2
  exit 1
fi
echo '{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
exit 0
`, counterFile, counterFile, counterFile)
	writeTestExecutable(t, mockBinary, mockScript)

	cp := NewCodexProvider(mockBinary, nil, map[string]string{TierLow: "gpt-4o-mini"})

	sleepCalled := false
	cp.sleepFn = func(ctx context.Context, d time.Duration) error {
		sleepCalled = true
		return nil
	}

	result, err := cp.StreamRun(context.Background(), "test", TierLow, nil, func([]byte) {}, nil)
	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("StreamRun() should succeed after retry, got %+v", result)
	}
	if !sleepCalled {
		t.Error("StreamRun() should have called sleepFn during retry backoff")
	}
}

func writeTestExecutable(t *testing.T, path, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create mock binary: %v", err)
	}
	// Explicitly sync filesystem to ensure file is readable before execution.
	// This prevents ETXTBSY ("text file busy") errors under parallel test load.
	syscall.Sync()
}

func readAttemptCount(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read attempt counter: %v", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("failed to parse attempt counter %q: %v", strings.TrimSpace(string(data)), err)
	}
	return count
}

// TestCodexProviderRunWithAdditionalFlags verifies that Run() includes
// any additional flags configured in the CodexProvider.flags field.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderRunWithAdditionalFlags(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `echo "ARGS: $@"
exit 0`)

	flags := []string{"--verbose", "--no-color"}
	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, flags, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	outputStr := result.Output
	if !strings.Contains(outputStr, "--verbose") {
		t.Errorf("Run() output missing --verbose flag, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "--no-color") {
		t.Errorf("Run() output missing --no-color flag, got: %s", outputStr)
	}
}

// TestCodexProviderNilReceiverSafety verifies that CodexProvider methods
// handle nil receiver safely by returning appropriate errors.
// Expected failure: CodexProvider methods do not exist yet
func TestCodexProviderNilReceiverSafety(t *testing.T) {
	t.Parallel()
	var cp *CodexProvider

	t.Run("Name with nil receiver", func(t *testing.T) {
		t.Parallel(
		// Name() should handle nil receiver safely (return empty string or panic)
		)

		defer func() {
			if r := recover(); r == nil {
				name := cp.Name()
				if name != "" && name != "codex" {
					t.Errorf("Name() on nil receiver = %q, want empty or 'codex'", name)
				}
			}
		}()
		_ = cp.Name()
	})

	t.Run("Run with nil receiver", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		result, err := cp.Run(ctx, "test", TierLow)

		if err == nil {
			t.Error("Run() on nil receiver error = nil, want non-nil error")
		}
		if result != nil {
			t.Errorf("Run() on nil receiver result = %v, want nil", result)
		}
	})
}

// TestCodexProviderBinaryNotFound verifies that Run() returns an appropriate
// error when the codex binary path does not exist.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderBinaryNotFound(t *testing.T) {
	t.Parallel()
	nonexistentPath := "/nonexistent/path/to/codex"
	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(nonexistentPath, []string{}, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err == nil {
		t.Error("Run() with nonexistent binary error = nil, want non-nil error")
	}

	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "executable") && !strings.Contains(errMsg, "not found") &&
			!strings.Contains(errMsg, "no such file") {
			t.Errorf("Run() error = %q, want error indicating binary not found", errMsg)
		}
	}

	// Result should be nil when command fails to start
	if result != nil {
		t.Errorf("Run() result = %v, want nil when binary not found", result)
	}
}

// TestCodexProviderEmptyTierToModelMap verifies that CodexProvider handles
// an empty tierToModel map by falling back to using the tier name as the model.
// Expected failure: CodexProvider Run() method does not exist yet
func TestCodexProviderEmptyTierToModelMap(t *testing.T) {
	t.Parallel()
	mockBinary := newTestBinary(t, `# Extract model value from arguments
MODEL=""
while [[ $# -gt 0 ]]; do
    if [ "$1" = "--model" ]; then
        MODEL="$2"
        break
    fi
    shift
done
echo "MODEL: $MODEL"
exit 0`)

	// Empty tier map
	emptyTierMap := map[string]string{}
	cp := NewCodexProvider(mockBinary, []string{}, emptyTierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	// When tier is not in map, should use tier name directly
	if !strings.Contains(result.Output, "MODEL: "+TierMedium) {
		t.Errorf("Run() with empty tier map should use tier name as model, got output: %s", result.Output)
	}
}

// TestCodexProviderIntegrationWithRealBinary is a comprehensive integration test
// that verifies the full CodexProvider flow with a real-like binary interaction.
// Expected failure: CodexProvider struct and methods do not exist yet
func TestCodexProviderIntegrationWithRealBinary(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Fatal("bash not available for integration test")
	}

	// Create a realistic mock codex binary that reads from stdin
	mockBinary := newTestBinary(t, `# Parse arguments
MODEL=""
JSON_MODE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --model)
            MODEL="$2"
            shift 2
            ;;
        --json)
            JSON_MODE=true
            shift
            ;;
        *)
            shift
            ;;
    esac
done

PROMPT=$(cat)

if [ "$JSON_MODE" = "true" ]; then
    # JSONL mode for StreamRun
    echo "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"Processing with model: $MODEL\nPrompt content:\n$PROMPT\nResponse: This is a simulated Codex response.\"}}"
    echo '{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}'
else
    # Plain text mode for Run
    echo "Processing with model: $MODEL"
    echo "Prompt content:"
    echo "$PROMPT"
    echo "Response: This is a simulated Codex response."
fi

exit 0`)

	// Create CodexProvider with full configuration
	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	// Verify Name()
	if name := cp.Name(); name != "codex" {
		t.Errorf("Name() = %q, want %q", name, "codex")
	}

	// Verify ModelForTier()
	if model := cp.ModelForTier(TierHigh); model != "o3" {
		t.Errorf("ModelForTier(TierHigh) = %q, want %q", model, "o3")
	}

	// Test Run()
	ctx := context.Background()
	testPrompt := "Write a function to calculate fibonacci numbers"
	result, err := cp.Run(ctx, testPrompt, TierMedium)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if !result.Success {
		t.Errorf("Run() Success = false, want true. Output: %s", result.Output)
	}

	if result.ExitCode != 0 {
		t.Errorf("Run() ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Model != "gpt-4o" {
		t.Errorf("Run() Model = %q, want %q", result.Model, "gpt-4o")
	}

	if !strings.Contains(result.Output, testPrompt) {
		t.Errorf("Run() output missing prompt content, got: %s", result.Output)
	}

	if !strings.Contains(result.Output, "gpt-4o") {
		t.Errorf("Run() output missing model name, got: %s", result.Output)
	}

	if result.Duration <= 0 {
		t.Errorf("Run() Duration = %v, want > 0", result.Duration)
	}

	// Test StreamRun()
	var streamOutput bytes.Buffer
	streamResult, err := cp.StreamRun(ctx, testPrompt, TierHigh, &streamOutput, nil, nil)

	if err != nil {
		t.Fatalf("StreamRun() error = %v, want nil", err)
	}

	if streamResult == nil {
		t.Fatal("StreamRun() returned nil result")
	}

	if streamResult.Model != "o3" {
		t.Errorf("StreamRun() Model = %q, want %q", streamResult.Model, "o3")
	}

	streamOutputStr := streamOutput.String()
	if !strings.Contains(streamOutputStr, testPrompt) {
		t.Errorf("StreamRun() streamed output missing prompt, got: %s", streamOutputStr)
	}
}

// TestCodexProviderMaxInputTokensConfig verifies that CodexProvider supports
// a max_input_tokens configuration that limits input token usage.
func TestCodexProviderMaxInputTokensConfig(t *testing.T) {
	t.Parallel()
	cp := NewCodexProvider("/usr/bin/codex", []string{}, map[string]string{TierMedium: "gpt-4o"})

	// SetMaxInputTokens should allow configuration of max input token threshold
	cp.SetMaxInputTokens(TierMedium, 2000000) // 2M token limit

	if cp.MaxInputTokensForTier(TierMedium) != 2000000 {
		t.Errorf("MaxInputTokensForTier() = %d, want 2000000", cp.MaxInputTokensForTier(TierMedium))
	}
}

// TestNewTestBinaryWithSync verifies that newTestBinary() helper function creates
// executable files with proper filesystem sync to prevent ETXTBSY errors under
// parallel test execution.
func TestNewTestBinaryWithSync(t *testing.T) {
	t.Parallel()
	bashScript := `cat > /dev/null
echo "Test output line 1"
echo "Test output line 2"
exit 0`

	mockBinary := newTestBinary(t, bashScript)

	tierMap := map[string]string{TierLow: "gpt-4o-mini"}
	cp := NewCodexProvider(mockBinary, []string{}, tierMap)

	ctx := context.Background()
	result, err := cp.Run(ctx, "test", TierLow)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if !strings.Contains(result.Output, "Test output line 1") {
		t.Errorf("Run() output missing expected stdout, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Test output line 2") {
		t.Errorf("Run() output missing expected stdout line 2, got: %s", result.Output)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		t.Errorf("Run() stderr should be empty for stdout-only command, got: %q", result.Stderr)
	}
}

// TestNewShellProviderCreatesShellBasedProvider verifies that newShellProvider()
// creates a CodexProvider that executes shell commands via /bin/sh -c pattern,
// avoiding temporary executable files and ETXTBSY errors under parallel execution.
func TestNewShellProviderCreatesShellBasedProvider(t *testing.T) {
	t.Parallel()
	bashScript := "echo done; exit 0"

	tierMap := map[string]string{
		TierHigh:   "o3",
		TierMedium: "gpt-4o",
		TierLow:    "gpt-4o-mini",
	}
	cp := newShellProvider(bashScript, tierMap)

	if cp == nil {
		t.Fatal("newShellProvider() returned nil")
	}

	tests := []struct {
		tier      string
		wantModel string
	}{
		{TierHigh, "o3"},
		{TierMedium, "gpt-4o"},
		{TierLow, "gpt-4o-mini"},
	}

	for _, tt := range tests {
		t.Run("tier_"+tt.tier, func(t *testing.T) {
			ctx := context.Background()
			result, err := cp.Run(ctx, "test", tt.tier)

			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}

			if result.Model != tt.wantModel {
				t.Errorf("Run() Model = %q, want %q for tier %s",
					result.Model, tt.wantModel, tt.tier)
			}
		})
	}
}

func TestCodexStreamRunOnceUsesKillDescendantsOnCancel(t *testing.T) {
	t.Parallel()

	script := `#!/bin/bash
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}'
printf '%s\n' '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1,"total_cost_usd":0}}'
exit 0
`
	mockBinary := testCreateBinaryWithETXTBSYProtection(t, script)
	cp := NewCodexProvider(mockBinary, []string{}, map[string]string{TierLow: "gpt-5.3-codex"})

	var killCalled bool
	oldKill := codexKillDescendantsOnCancelFn
	t.Cleanup(func() { codexKillDescendantsOnCancelFn = oldKill })
	codexKillDescendantsOnCancelFn = func(ctx context.Context, cmd *exec.Cmd) {
		killCalled = true
	}

	ctx := context.Background()
	var output bytes.Buffer
	result, err := cp.streamRunOnce(ctx, "test prompt", TierLow, "", &output, nil, nil)
	if err != nil {
		t.Fatalf("streamRunOnce() error = %v", err)
	}
	if result == nil {
		t.Fatal("streamRunOnce() returned nil result")
	}
	if !killCalled {
		t.Fatal("streamRunOnce() did not call KillDescendantsOnCancel")
	}
}
