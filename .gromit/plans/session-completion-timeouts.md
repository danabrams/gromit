---
id: session-completion-timeouts
source_spec: session-completion-timeouts
created: 2026-02-19
decomposed: true
---

# Session Completion Timeouts Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add configurable timeout to `git pull --rebase` and `git push` in `runSessionCompletion()` so hung network connections resolve within 60 seconds instead of blocking indefinitely.

**Architecture:** Add `PushTimeout int` to `GitConfig` (default 60s, 0 disables). In `runSessionCompletion()`, build a `context.WithTimeout` for the two network-facing git calls. Timeout expiry follows the existing `push_failure` policy. Local git ops keep `context.Background()`.

**Tech Stack:** Go, YAML config

**Spec:** `.gromit/specs/session-completion-timeouts.md`

---

## Architecture

**Overview:** Single new config field + two context replacements in lifecycle.go.

**Key Components:**
1. **`GitConfig.PushTimeout`** — `int` field (seconds), YAML tag `push_timeout`, default 60 in `SetDefaults()`
2. **`GitConfig.PushTimeoutDuration()`** — Accessor returning `time.Duration`; 0 means disabled
3. **`runSessionCompletion()` timeout** — Replace `context.Background()` with `context.WithTimeout` for `git pull --rebase` and `git push`

**Integration Points:**
- Existing `runCmd` delegates to `exec.CommandContext`, so timeout context kills the subprocess correctly
- Timeout failure is just another push failure — reuses `push_failure` policy ("stop" returns error, "warn" logs warning)
- Local ops (`commitGeneratedMetrics`, `commitGeneratedState`, `git status`) keep `context.Background()`

**Files to Modify:**
- `internal/config/config.go` — Add field, default, accessor
- `internal/runner/lifecycle.go` — Use timeout context for pull/push
- `gromit.yaml` — Document new field

**Tradeoffs:**
- **Fresh timeout vs context propagation**: Parent context may already be cancelled (Ctrl+C). Fresh timeout lets push attempt on its own terms.
- **Single field for both pull and push**: Same failure domain (network). One knob is simpler.

## Test Strategy

**Unit Tests (config):**
- `PushTimeout` defaults to 60 when unset
- `PushTimeout` deserializes from YAML
- `PushTimeoutDuration()` returns correct duration for default/custom/zero

**Unit Tests (lifecycle):**
- Mock `cmdRunnerFn` inspects context for deadline presence
- `git pull --rebase` and `git push` receive context with deadline
- `context.DeadlineExceeded` with `push_failure: "stop"` returns error
- `context.DeadlineExceeded` with `push_failure: "warn"` logs warning, returns nil
- `push_timeout: 0` disables timeout (no deadline on context)
- Local ops still use `context.Background()` (no deadline)

**Mocking Strategy:**
- Existing `cmdRunnerFn` mock — inspect `ctx` argument for deadline
- No new mocks needed

## Implementation Tasks

### Task 1: Add PushTimeout field to GitConfig with default and accessor

**Files:**
- Modify: `internal/config/config.go:256-259` (GitConfig struct)
- Modify: `internal/config/config.go:674-680` (SetDefaults)
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test for PushTimeout default**

```go
func TestGitPushTimeoutDefault(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	os.WriteFile(cfgPath, []byte("models:\n  p0: opus\n"), 0644)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Git.PushTimeout != 60 {
		t.Errorf("expected default PushTimeout=60, got %d", cfg.Git.PushTimeout)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestGitPushTimeoutDefault -v`
Expected: FAIL — PushTimeout field doesn't exist yet

**Step 3: Add PushTimeout field to GitConfig and default in SetDefaults**

In `internal/config/config.go`, add `PushTimeout int` with YAML tag `push_timeout` to `GitConfig`. In `SetDefaults()`, add `if c.Git.PushTimeout == 0 { c.Git.PushTimeout = 60 }`.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestGitPushTimeoutDefault -v`
Expected: PASS

**Step 5: Write the failing test for YAML deserialization**

```go
func TestGitPushTimeoutFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := "models:\n  p0: opus\ngit:\n  push_timeout: 30\n"
	os.WriteFile(cfgPath, []byte(yaml), 0644)
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Git.PushTimeout != 30 {
		t.Errorf("expected PushTimeout=30, got %d", cfg.Git.PushTimeout)
	}
}
```

**Step 6: Run test — should pass (YAML tag wiring already done)**

Run: `go test ./internal/config/ -run TestGitPushTimeoutFromYAML -v`
Expected: PASS

**Step 7: Write the failing test for PushTimeoutDuration accessor**

```go
func TestGitPushTimeoutDuration(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
		want    time.Duration
	}{
		{"Default", 60, 60 * time.Second},
		{"Custom", 30, 30 * time.Second},
		{"Disabled", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GitConfig{PushTimeout: tt.timeout}
			if got := cfg.PushTimeoutDuration(); got != tt.want {
				t.Errorf("PushTimeoutDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 8: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestGitPushTimeoutDuration -v`
Expected: FAIL — method doesn't exist

**Step 9: Add PushTimeoutDuration method**

```go
// PushTimeoutDuration returns the push timeout as a time.Duration.
// Zero means disabled (no timeout).
func (g GitConfig) PushTimeoutDuration() time.Duration {
	return time.Duration(g.PushTimeout) * time.Second
}
```

**Step 10: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestGitPushTimeoutDuration -v`
Expected: PASS

**Step 11: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add PushTimeout field to GitConfig with 60s default"
```

**Acceptance Criteria:**
- `PushTimeout` defaults to 60 when not set in YAML
- YAML `push_timeout: 30` deserializes correctly
- `PushTimeoutDuration()` returns correct `time.Duration` for default, custom, and zero

---

### Task 2: Use timeout context for git pull --rebase and git push in runSessionCompletion

**Files:**
- Modify: `internal/runner/lifecycle.go:202-278`
- Test: `internal/runner/runner_test.go`

**Dependencies:** Task 1

**Step 1: Write the failing test — pull/push receive context with deadline**

```go
func TestRunSessionCompletion_UsesTimeoutContext(t *testing.T) {
	autoPush := true
	cfg := &config.Config{
		Git: config.GitConfig{
			AutoPush:    &autoPush,
			PushFailure: "warn",
			PushTimeout: 60,
		},
	}
	var pullHadDeadline, pushHadDeadline bool
	r := &Runner{
		cfg:    cfg,
		output: &strings.Builder{},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			if command == "git pull --rebase" {
				_, pullHadDeadline = ctx.Deadline()
			}
			if command == "git push" {
				_, pushHadDeadline = ctx.Deadline()
			}
			return "", "", 0, nil
		},
	}
	if err := r.runSessionCompletion(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pullHadDeadline {
		t.Error("git pull --rebase context should have a deadline")
	}
	if !pushHadDeadline {
		t.Error("git push context should have a deadline")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestRunSessionCompletion_UsesTimeoutContext -v`
Expected: FAIL — pull/push still use `context.Background()` which has no deadline

**Step 3: Implement timeout context in runSessionCompletion**

In `lifecycle.go`, add a helper at the top of `runSessionCompletion()` to build the timeout context:

```go
// networkCtx returns a context with the configured push timeout.
// If timeout is 0 (disabled), returns context.Background().
networkCtx := func() (context.Context, context.CancelFunc) {
	d := r.cfg.Git.PushTimeoutDuration()
	if d == 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), d)
}
```

Replace `context.Background()` in the `git pull --rebase` call (line 215) with `networkCtx()`, and in the `git push` call (line 259) with `networkCtx()`. Call `defer cancel()` for each.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/runner/ -run TestRunSessionCompletion_UsesTimeoutContext -v`
Expected: PASS

**Step 5: Write the failing test — timeout with push_failure "stop" returns error**

```go
func TestRunSessionCompletion_TimeoutReturnsErrorOnStop(t *testing.T) {
	autoPush := true
	cfg := &config.Config{
		Git: config.GitConfig{
			AutoPush:    &autoPush,
			PushFailure: "stop",
			PushTimeout: 60,
		},
	}
	r := &Runner{
		cfg:    cfg,
		output: &strings.Builder{},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			if command == "git push" {
				return "", "", 0, context.DeadlineExceeded
			}
			return "", "", 0, nil
		},
	}
	err := r.runSessionCompletion()
	if err == nil {
		t.Fatal("expected error when push times out with push_failure=stop")
	}
	if !strings.Contains(err.Error(), "git push failed") {
		t.Errorf("expected 'git push failed' in error, got: %v", err)
	}
}
```

**Step 6: Run test — should pass (existing error handling covers this)**

Run: `go test ./internal/runner/ -run TestRunSessionCompletion_TimeoutReturnsErrorOnStop -v`
Expected: PASS — the existing `if err != nil` branch already handles `context.DeadlineExceeded`

**Step 7: Write the failing test — timeout with push_failure "warn" logs warning**

```go
func TestRunSessionCompletion_TimeoutWarnsOnWarn(t *testing.T) {
	autoPush := true
	cfg := &config.Config{
		Git: config.GitConfig{
			AutoPush:    &autoPush,
			PushFailure: "warn",
			PushTimeout: 60,
		},
	}
	var buf strings.Builder
	r := &Runner{
		cfg:    cfg,
		output: &buf,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			if command == "git push" {
				return "", "", 0, context.DeadlineExceeded
			}
			return "", "", 0, nil
		},
	}
	err := r.runSessionCompletion()
	if err != nil {
		t.Fatalf("expected nil error with push_failure=warn, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Warning") {
		t.Error("expected warning in output")
	}
}
```

**Step 8: Run test — should pass (existing warn path handles this)**

Run: `go test ./internal/runner/ -run TestRunSessionCompletion_TimeoutWarnsOnWarn -v`
Expected: PASS

**Step 9: Write the failing test — push_timeout 0 disables timeout**

```go
func TestRunSessionCompletion_ZeroTimeoutDisabled(t *testing.T) {
	autoPush := true
	cfg := &config.Config{
		Git: config.GitConfig{
			AutoPush:    &autoPush,
			PushFailure: "warn",
			PushTimeout: 0,
		},
	}
	var pullHadDeadline, pushHadDeadline bool
	r := &Runner{
		cfg:    cfg,
		output: &strings.Builder{},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			if command == "git pull --rebase" {
				_, pullHadDeadline = ctx.Deadline()
			}
			if command == "git push" {
				_, pushHadDeadline = ctx.Deadline()
			}
			return "", "", 0, nil
		},
	}
	if err := r.runSessionCompletion(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pullHadDeadline {
		t.Error("git pull --rebase should NOT have deadline when timeout=0")
	}
	if pushHadDeadline {
		t.Error("git push should NOT have deadline when timeout=0")
	}
}
```

**Step 10: Run test — should pass (networkCtx returns context.Background() for 0)**

Run: `go test ./internal/runner/ -run TestRunSessionCompletion_ZeroTimeoutDisabled -v`
Expected: PASS

**Step 11: Write the failing test — local ops have no deadline**

```go
func TestRunSessionCompletion_LocalOpsNoDeadline(t *testing.T) {
	autoPush := true
	cfg := &config.Config{
		Git: config.GitConfig{
			AutoPush:    &autoPush,
			PushFailure: "warn",
			PushTimeout: 60,
		},
	}
	localCmdsWithDeadline := []string{}
	localPrefixes := []string{metricsStatusCommand, metricsAddCommand, metricsCommitCommand,
		stateStatusCommand, stateAddCommand, stateCommitCommand}
	r := &Runner{
		cfg:    cfg,
		output: &strings.Builder{},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			for _, prefix := range localPrefixes {
				if command == prefix {
					if _, ok := ctx.Deadline(); ok {
						localCmdsWithDeadline = append(localCmdsWithDeadline, command)
					}
				}
			}
			return "", "", 0, nil
		},
	}
	if err := r.runSessionCompletion(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(localCmdsWithDeadline) > 0 {
		t.Errorf("local ops should not have deadline, but these did: %v", localCmdsWithDeadline)
	}
}
```

**Step 12: Run test — should pass (local ops still use context.Background())**

Run: `go test ./internal/runner/ -run TestRunSessionCompletion_LocalOpsNoDeadline -v`
Expected: PASS

**Step 13: Run full test suite**

Run: `go test ./internal/runner/ ./internal/config/ -v -count=1`
Expected: All tests PASS

**Step 14: Commit**

```bash
git add internal/runner/lifecycle.go internal/runner/runner_test.go
git commit -m "feat: add timeout context for git pull/push in session completion"
```

**Acceptance Criteria:**
- `git pull --rebase` and `git push` receive context with deadline when `PushTimeout > 0`
- `context.DeadlineExceeded` follows `push_failure` policy (stop → error, warn → warning)
- `push_timeout: 0` disables timeout (no deadline)
- Local git ops (`commitGeneratedMetrics`, `commitGeneratedState`) have no deadline

---

### Task 3: Document push_timeout in gromit.yaml

**Files:**
- Modify: `gromit.yaml:185-187`

**Dependencies:** Task 1

**Step 1: Add push_timeout to gromit.yaml git section**

```yaml
git:
  auto_push: true  # Push to remote after each successful bead (default: true)
  push_failure: "warn"  # What to do when push fails: "warn" | "stop" (default: "warn")
  push_timeout: 60  # seconds; 0 disables the timeout (default: 60)
```

**Step 2: Verify config loads**

Run: `go test ./internal/config/ -run TestGitPushTimeout -v`
Expected: PASS

**Step 3: Run full quality gates**

Run: `go test ./... && go vet ./... && go build ./...`
Expected: All pass

**Step 4: Commit**

```bash
git add gromit.yaml
git commit -m "docs: add push_timeout to gromit.yaml git section"
```

**Acceptance Criteria:**
- `gromit.yaml` documents `push_timeout` with comment explaining default and disable behavior

---

## Notes

- The `git pull --rebase` retry loop (up to `SessionCompletionRebaseRetryCount` attempts) gets a fresh timeout context per attempt. Each attempt has the full timeout budget — not a shared budget across retries.
- The verification `git status --short --branch` call (line 275) keeps `context.Background()` since it's local and best-effort.
- `context.DeadlineExceeded` is a regular error, so the existing error-handling branches in `runSessionCompletion()` handle it without special-casing.
