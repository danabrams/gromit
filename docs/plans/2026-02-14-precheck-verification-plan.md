# Precheck Two-Model Verification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a sonnet verification phase to precheck so that haiku false positives are caught before beads are auto-closed.

**Architecture:** When haiku's precheck says PRECHECK_PASSED, run the same prompt through sonnet independently. Both must agree for auto-close. The change is contained to config (new struct + defaults) and `runPrecheck()` (phase 2 logic). The main loop is untouched.

**Tech Stack:** Go, existing provider/router infrastructure

---

### Task 1: Add VerificationConfig to PrecheckConfig

**Files:**
- Modify: `internal/config/config.go:82-86` (PrecheckConfig struct)
- Modify: `internal/config/config.go:348-357` (SetDefaults)
- Modify: `internal/config/config.go:218-263` (NormalizeNilFields — no new slices/maps needed, but add Verification defaults)

**Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestPrecheckVerificationDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Precheck.Verification.Enabled == nil {
		t.Fatal("Verification.Enabled should not be nil after SetDefaults")
	}
	if !*cfg.Precheck.Verification.Enabled {
		t.Error("Verification.Enabled should default to true")
	}
	if cfg.Precheck.Verification.TimeoutSeconds != 120 {
		t.Errorf("Verification.TimeoutSeconds should default to 120, got %d", cfg.Precheck.Verification.TimeoutSeconds)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestPrecheckVerificationDefaults -v`
Expected: FAIL — VerificationConfig type does not exist

**Step 3: Write minimal implementation**

In `internal/config/config.go`, add the VerificationConfig struct and nest it in PrecheckConfig:

```go
type PrecheckVerificationConfig struct {
	Enabled        *bool  `yaml:"enabled"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type PrecheckConfig struct {
	Enabled        *bool                       `yaml:"enabled"`
	Model          string                      `yaml:"model"`
	TimeoutSeconds int                         `yaml:"timeout_seconds"`
	Verification   PrecheckVerificationConfig  `yaml:"verification"`
}
```

Add convenience method:

```go
// IsVerificationEnabled returns whether precheck verification should run (defaults to true)
func (v PrecheckVerificationConfig) IsVerificationEnabled() bool {
	if v.Enabled == nil {
		return true
	}
	return *v.Enabled
}
```

In `SetDefaults()`, after the existing precheck defaults (line ~357):

```go
if c.Precheck.Verification.Enabled == nil {
	t := true
	c.Precheck.Verification.Enabled = &t
}
if c.Precheck.Verification.TimeoutSeconds == 0 {
	c.Precheck.Verification.TimeoutSeconds = 120
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestPrecheckVerificationDefaults -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./internal/config/ -v`
Expected: All tests pass — no regressions. The existing project-config integration test parses gromit.yaml; the new nested struct with zero-value defaults won't break existing YAML parsing.

**Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add PrecheckVerificationConfig with defaults"
```

---

### Task 2: Update gromit.yaml with verification section

**Files:**
- Modify: `gromit.yaml:87-91` (precheck section)

**Step 1: Update the config file**

Change the precheck section in `gromit.yaml` from:

```yaml
precheck:
  enabled: true
  model: haiku
  timeout_seconds: 120
```

to:

```yaml
precheck:
  enabled: true
  model: haiku
  timeout_seconds: 120
  verification:
    enabled: true
    timeout_seconds: 120
```

**Step 2: Run config integration test**

Run: `go test ./internal/config/ -v`
Expected: All pass — the project-config integration test validates that gromit.yaml parses correctly with the current schema.

**Step 3: Commit**

```bash
git add gromit.yaml
git commit -m "config: add precheck verification section to gromit.yaml"
```

---

### Task 3: Add verification phase to runPrecheck

**Files:**
- Modify: `internal/runner/runner.go:1918-1987` (runPrecheck method)

**Step 1: Write the failing test — verification rejects**

Add to `internal/runner/interfaces_test.go`:

```go
func TestRunWithMocks_PrecheckVerificationRejects(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "verify-reject",
				Title:           "Needs work",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature implemented"},
			}, nil
		},
	}

	runCallCount := 0
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			runCallCount++
			if runCallCount == 1 {
				// Phase 1 (haiku/low tier): PASSED
				return &claude.Result{Success: true, Output: "PRECHECK_PASSED"}, nil
			}
			// Phase 2 (sonnet/medium tier): NOT MET
			return &claude.Result{Success: true, Output: "PRECHECK_NOT_MET"}, nil
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler func(line []byte), onToolCall func(toolName, filePath string)) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "Build complete"}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	verifyEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:   config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{
				Enabled:        &precheckEnabled,
				Model:          "haiku",
				TimeoutSeconds: 30,
				Verification: config.PrecheckVerificationConfig{
					Enabled:        &verifyEnabled,
					TimeoutSeconds: 30,
				},
			},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Bead should NOT have been closed — verification rejected
	if len(beads.ClosedIDs) != 0 {
		t.Errorf("expected no beads closed, got: %v", beads.ClosedIDs)
	}

	// Should have gone through normal build (StreamRun called)
	if len(mockClaude.StreamRunCalls) == 0 {
		t.Error("expected StreamRun to be called for normal build after verification rejection")
	}

	// Console should show verification rejection
	output := buf.String()
	if !strings.Contains(output, "verification rejected") {
		t.Errorf("expected verification rejection message in output, got: %s", output)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestRunWithMocks_PrecheckVerificationRejects -v`
Expected: FAIL — runPrecheck doesn't have phase 2 yet, so it auto-closes on the first PRECHECK_PASSED

**Step 3: Write the failing test — verification confirms**

Add to `internal/runner/interfaces_test.go`:

```go
func TestRunWithMocks_PrecheckVerificationConfirms(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "verify-confirm",
				Title:           "Already done",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature implemented"},
			}, nil
		},
	}

	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			// Both phase 1 and phase 2 return PASSED
			return &claude.Result{Success: true, Output: "PRECHECK_PASSED"}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	verifyEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:   config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{
				Enabled:        &precheckEnabled,
				Model:          "haiku",
				TimeoutSeconds: 30,
				Verification: config.PrecheckVerificationConfig{
					Enabled:        &verifyEnabled,
					TimeoutSeconds: 30,
				},
			},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Bead SHOULD be closed — both phases agreed
	if len(beads.ClosedIDs) != 1 || beads.ClosedIDs[0] != "verify-confirm" {
		t.Errorf("expected bead 'verify-confirm' to be closed, got: %v", beads.ClosedIDs)
	}

	// Should have 2 Run calls (phase 1 + phase 2) and 0 StreamRun calls (no build)
	if len(mockClaude.RunCalls) != 2 {
		t.Errorf("expected 2 Run calls (phase 1 + phase 2), got %d", len(mockClaude.RunCalls))
	}
	if len(mockClaude.StreamRunCalls) != 0 {
		t.Errorf("expected 0 StreamRun calls, got %d", len(mockClaude.StreamRunCalls))
	}
}
```

**Step 4: Write the failing test — verification error falls back to build**

Add to `internal/runner/interfaces_test.go`:

```go
func TestRunWithMocks_PrecheckVerificationError(t *testing.T) {
	callCount := 0
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			callCount++
			if callCount > 1 {
				return nil, nil
			}
			return &bead.Bead{
				ID:              "verify-error",
				Title:           "Check fails",
				Priority:        1,
				Labels:          []string{},
				ExpectedOutputs: []string{"feature implemented"},
			}, nil
		},
	}

	runCallCount := 0
	mockClaude := &mockClaudeClient{
		RunFn: func(ctx context.Context, prompt string, model string) (*claude.Result, error) {
			runCallCount++
			if runCallCount == 1 {
				// Phase 1: PASSED
				return &claude.Result{Success: true, Output: "PRECHECK_PASSED"}, nil
			}
			// Phase 2: error
			return nil, fmt.Errorf("provider unavailable")
		},
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler func(line []byte), onToolCall func(toolName, filePath string)) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "Build complete"}, nil
		},
	}

	mockLog := &mockIterationLogger{}

	precheckEnabled := true
	verifyEnabled := true
	var buf strings.Builder
	r, _ := NewRunnerWithDeps(
		&config.Config{
			Claude:   config.ClaudeConfig{BeadTimeout: 60},
			Precheck: config.PrecheckConfig{
				Enabled:        &precheckEnabled,
				Model:          "haiku",
				TimeoutSeconds: 30,
				Verification: config.PrecheckVerificationConfig{
					Enabled:        &verifyEnabled,
					TimeoutSeconds: 30,
				},
			},
		},
		&buf, t.TempDir(),
		Deps{Beads: beads, Router: newMockRouterFromClaudeClient(mockClaude), Analyzer: &mockFailureAnalyzer{}, Renderer: &mockPromptRenderer{}, Logger: mockLog})

	if err := r.Run(context.Background(), 5, time.Time{}, false); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Bead should NOT be closed — verification errored, conservative fallback
	if len(beads.ClosedIDs) != 0 {
		t.Errorf("expected no beads closed, got: %v", beads.ClosedIDs)
	}

	// Should have proceeded to normal build
	if len(mockClaude.StreamRunCalls) == 0 {
		t.Error("expected StreamRun to be called for normal build after verification error")
	}
}
```

**Step 5: Run all three tests to verify they fail**

Run: `go test ./internal/runner/ -run "TestRunWithMocks_PrecheckVerification" -v`
Expected: TestRunWithMocks_PrecheckVerificationRejects FAIL (bead gets closed without verification), TestRunWithMocks_PrecheckVerificationConfirms PASS (both return PASSED, existing logic closes it), TestRunWithMocks_PrecheckVerificationError FAIL (error in phase 2 not handled)

**Step 6: Implement verification in runPrecheck**

Modify `runPrecheck()` in `internal/runner/runner.go`. After the existing `passed` check (line 1978), add the verification phase:

```go
func (r *Runner) runPrecheck(ctx context.Context, b *bead.Bead) (bool, time.Duration) {
	start := time.Now()

	if r == nil || b == nil || r.cfg == nil || r.renderer == nil || r.router == nil {
		return false, 0
	}

	// Check if precheck is enabled
	if !r.cfg.Precheck.IsEnabled() {
		return false, 0
	}

	// Get parent bead if exists
	parent, err := r.beads.GetParent(b)
	if err != nil {
		r.log("Warning: failed to get parent bead for precheck: %v", err)
	}

	// Build precheck context
	precheckCtx := &prompt.PrecheckContext{
		Bead:       b,
		ParentBead: parent,
	}

	// Render precheck prompt
	precheckPrompt, err := r.renderer.RenderPrecheck(precheckCtx)
	if err != nil {
		r.log("Warning: failed to render precheck prompt: %v", err)
		return false, time.Since(start)
	}

	// Phase 1: Screen with low tier (haiku)
	precheckTimeout := time.Duration(r.cfg.Precheck.TimeoutSeconds) * time.Second
	precheckCtx2, cancel := context.WithTimeout(ctx, precheckTimeout)
	defer cancel()

	p, _ := r.router.Select("precheck", provider.TierLow)
	if p == nil {
		r.log("Warning: no provider available for precheck")
		return false, time.Since(start)
	}

	result, err := p.Run(precheckCtx2, precheckPrompt, provider.TierLow)
	if err != nil {
		r.log("Warning: precheck invocation failed: %v", err)
		return false, time.Since(start)
	}
	if result == nil {
		r.log("Warning: precheck returned nil result")
		return false, time.Since(start)
	}

	if !result.Success {
		r.log("Warning: precheck failed with exit code %d", result.ExitCode)
		return false, time.Since(start)
	}

	passed := strings.Contains(result.Output, "PRECHECK_PASSED")

	if !passed {
		r.log("Pre-check: acceptance criteria not yet met")
		return false, time.Since(start)
	}

	// Phase 1 passed — run verification if enabled
	if !r.cfg.Precheck.Verification.IsVerificationEnabled() {
		r.log("Pre-check: acceptance criteria already met")
		return true, time.Since(start)
	}

	// Phase 2: Verify with medium tier (sonnet)
	r.log("Pre-check: phase 1 passed, running verification")
	verifyTimeout := time.Duration(r.cfg.Precheck.Verification.TimeoutSeconds) * time.Second
	verifyCtx, verifyCancel := context.WithTimeout(ctx, verifyTimeout)
	defer verifyCancel()

	vp, _ := r.router.Select("precheck", provider.TierMedium)
	if vp == nil {
		r.log("Warning: no provider available for precheck verification, proceeding to build")
		return false, time.Since(start)
	}

	verifyResult, err := vp.Run(verifyCtx, precheckPrompt, provider.TierMedium)
	if err != nil {
		r.log("Warning: precheck verification failed: %v, proceeding to build", err)
		return false, time.Since(start)
	}
	if verifyResult == nil {
		r.log("Warning: precheck verification returned nil result, proceeding to build")
		return false, time.Since(start)
	}

	if !verifyResult.Success {
		r.log("Warning: precheck verification exited with code %d, proceeding to build", verifyResult.ExitCode)
		return false, time.Since(start)
	}

	verified := strings.Contains(verifyResult.Output, "PRECHECK_PASSED")

	if verified {
		r.log("Pre-check: acceptance criteria already met (verified)")
	} else {
		r.log("Pre-check: phase 1 passed but verification rejected, proceeding to build")
	}

	return verified, time.Since(start)
}
```

**Step 7: Run all precheck tests**

Run: `go test ./internal/runner/ -run "TestRunWithMocks_Precheck" -v`
Expected: All pass, including the three new verification tests

**Step 8: Update existing PrecheckPassed test**

The existing `TestRunWithMocks_PrecheckPassed` test (line 1315) expects exactly 1 `RunCalls`. With verification enabled by default, it will now make 2 Run calls (phase 1 + phase 2). Update the assertion:

Change line ~1397-1399 from:
```go
if len(mockClaude.RunCalls) != 1 {
    t.Errorf("expected 1 Claude.Run call (precheck), got %d", len(mockClaude.RunCalls))
}
```
to:
```go
if len(mockClaude.RunCalls) != 2 {
    t.Errorf("expected 2 Claude.Run calls (precheck + verification), got %d", len(mockClaude.RunCalls))
}
```

Also update the `Model` assertion in the log check — with verification, the model logged should reflect the verifier. Check what `runPrecheck` logs. Since the iteration log at line 616 hardcodes `Model: "haiku"`, update it to log the actual model used. In the main loop (runner.go:615-616), change:
```go
Model: "haiku",
```
to:
```go
Model: "precheck",
```

This is a display-only field in iteration logs — "precheck" is more accurate than hardcoding "haiku" now that two models are involved.

**Step 9: Run full test suite**

Run: `go test ./internal/runner/ -v`
Expected: All pass

Run: `go test ./... `
Expected: All pass

**Step 10: Commit**

```bash
git add internal/runner/runner.go internal/runner/interfaces_test.go
git commit -m "feat: add two-model precheck verification (haiku screens, sonnet verifies)"
```

---

### Task 4: Final validation

**Step 1: Build the binary**

Run: `go build ./cmd/gromit`
Expected: Clean build

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: All pass

**Step 3: Run vet**

Run: `go vet ./...`
Expected: Clean

**Step 4: Verify config round-trip**

Run: `go run ./cmd/gromit status` (or any command that loads config)
Expected: No config parsing errors

**Step 5: Commit any final fixes if needed**
