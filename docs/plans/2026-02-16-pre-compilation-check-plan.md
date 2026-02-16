# Pre-Compilation Check Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Run `go build ./...` before each Claude invocation and inject any compilation errors into the build prompt, so the agent can fix them alongside its work.

**Architecture:** A new `runCompilationCheck` method on Runner runs `go build ./...` with a 30s timeout. If it fails, the error output is appended to `bc.BuildPrompt` in a `<compilation-errors>` section. A new `CompileCheck` bool in `PreflightConfig` controls the feature (default true). A `CompilationErrors` field on `IterationResult` and `IterationLog` tracks whether errors were present.

**Tech Stack:** Go stdlib (`os/exec`, `context`), existing runner/config/logger packages.

---

### Task 1: Add CompileCheck config field

**Files:**
- Modify: `internal/config/config.go:103-106` (PreflightConfig struct)
- Modify: `internal/config/config.go` (SetDefaults method)
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test**

In `internal/config/config_test.go`, add:

```go
func TestSetDefaults_CompileCheckDefaultsTrue(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.Preflight.CompileCheck == nil || !*cfg.Preflight.CompileCheck {
		t.Fatal("expected Preflight.CompileCheck to default to true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestSetDefaults_CompileCheck -v`
Expected: FAIL — `CompileCheck` field does not exist

**Step 3: Write minimal implementation**

In `internal/config/config.go`, update `PreflightConfig`:

```go
type PreflightConfig struct {
	AutoInstall  string   `yaml:"auto_install"`
	Tools        []string `yaml:"tools"`
	CompileCheck *bool    `yaml:"compile_check"`
}
```

In `SetDefaults()`, add after existing preflight defaults:

```go
if c.Preflight.CompileCheck == nil {
	t := true
	c.Preflight.CompileCheck = &t
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestSetDefaults_CompileCheck -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "Add CompileCheck config field to PreflightConfig (default true)"
```

---

### Task 2: Add CompilationErrors field to IterationResult and IterationLog

**Files:**
- Modify: `internal/runner/runtypes/types.go:55-89` (IterationResult struct)
- Modify: `internal/logger/logger.go:13-46` (IterationLog struct)
- Modify: `internal/runner/logging.go:37-67` (writeIterationLog mapping)

**Step 1: Add field to IterationResult**

In `internal/runner/runtypes/types.go`, add to IterationResult after `ValidationMode`:

```go
CompilationErrors bool   `json:"compilation_errors,omitempty"` // true when pre-build compilation check found errors
```

**Step 2: Add field to IterationLog**

In `internal/logger/logger.go`, add to IterationLog after `ValidationMode`:

```go
CompilationErrors bool   `json:"compilation_errors,omitempty"`
```

**Step 3: Wire field in writeIterationLog**

In `internal/runner/logging.go`, add to the `logger.IterationLog` literal in `writeIterationLog`:

```go
CompilationErrors:         result.CompilationErrors,
```

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: exit 0

**Step 5: Commit**

```bash
git add internal/runner/runtypes/types.go internal/logger/logger.go internal/runner/logging.go
git commit -m "Add CompilationErrors tracking field to iteration result and log"
```

---

### Task 3: Implement runCompilationCheck method

**Files:**
- Modify: `internal/runner/process.go` (add new method)
- Test: `internal/runner/process_test.go` (or new file `internal/runner/compilation_check_test.go`)

**Step 1: Write the failing test — compilation errors appended to prompt**

Create `internal/runner/compilation_check_test.go`:

```go
package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestRunCompilationCheck_ErrorsAppendedToPrompt(t *testing.T) {
	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Preflight: config.PreflightConfig{CompileCheck: &enabled},
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			if strings.Contains(command, "go build") {
				return "", "internal/foo/bar.go:10: undefined: SomeSymbol", 1, nil
			}
			return "", "", 0, nil
		},
	}

	bc := &runtypes.BeadContext{
		BuildPrompt: "original prompt",
		Result:      &runtypes.IterationResult{},
	}

	r.runCompilationCheck(context.Background(), bc)

	if !strings.Contains(bc.BuildPrompt, "<compilation-errors>") {
		t.Fatal("expected compilation errors section in prompt")
	}
	if !strings.Contains(bc.BuildPrompt, "undefined: SomeSymbol") {
		t.Fatal("expected error output in prompt")
	}
	if !bc.Result.CompilationErrors {
		t.Fatal("expected CompilationErrors flag to be true")
	}
}
```

**Step 2: Write the failing test — no errors leaves prompt unchanged**

In the same file:

```go
func TestRunCompilationCheck_NoErrors(t *testing.T) {
	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Preflight: config.PreflightConfig{CompileCheck: &enabled},
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "", 0, nil
		},
	}

	bc := &runtypes.BeadContext{
		BuildPrompt: "original prompt",
		Result:      &runtypes.IterationResult{},
	}

	r.runCompilationCheck(context.Background(), bc)

	if bc.BuildPrompt != "original prompt" {
		t.Fatalf("expected prompt unchanged, got %q", bc.BuildPrompt)
	}
	if bc.Result.CompilationErrors {
		t.Fatal("expected CompilationErrors flag to be false")
	}
}
```

**Step 3: Write the failing test — disabled config skips check**

```go
func TestRunCompilationCheck_Disabled(t *testing.T) {
	disabled := false
	r := &Runner{
		cfg: &config.Config{
			Preflight: config.PreflightConfig{CompileCheck: &disabled},
		},
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			t.Fatal("should not be called when disabled")
			return "", "", 0, nil
		},
	}

	bc := &runtypes.BeadContext{
		BuildPrompt: "original prompt",
		Result:      &runtypes.IterationResult{},
	}

	r.runCompilationCheck(context.Background(), bc)

	if bc.BuildPrompt != "original prompt" {
		t.Fatal("expected prompt unchanged when disabled")
	}
}
```

**Step 4: Run tests to verify they fail**

Run: `go test ./internal/runner/ -run TestRunCompilationCheck -v`
Expected: FAIL — `runCompilationCheck` method does not exist

**Step 5: Implement runCompilationCheck**

In `internal/runner/process.go`, add:

```go
// runCompilationCheck runs "go build ./..." before Claude invocation.
// If compilation fails, the errors are appended to the build prompt so the
// agent can fix them. Non-blocking: never prevents the bead from proceeding.
func (r *Runner) runCompilationCheck(ctx context.Context, bc *runtypes.BeadContext) {
	if r.cfg.Preflight.CompileCheck != nil && !*r.cfg.Preflight.CompileCheck {
		return
	}

	buildCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, stderr, exitCode, _ := r.cmdRunnerFn(buildCtx, "go build ./...", ".")
	if exitCode == 0 {
		return
	}

	r.log("Pre-build compilation check found errors, injecting into prompt")
	bc.Result.CompilationErrors = true
	bc.BuildPrompt += fmt.Sprintf("\n\n<compilation-errors>\nThe codebase currently has compilation errors. You must fix these as part of your work:\n\n%s\n</compilation-errors>", stderr)
}
```

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/runner/ -run TestRunCompilationCheck -v`
Expected: PASS (3 tests)

**Step 7: Commit**

```bash
git add internal/runner/process.go internal/runner/compilation_check_test.go
git commit -m "Implement runCompilationCheck: inject build errors into prompt"
```

---

### Task 4: Wire runCompilationCheck into processBead

**Files:**
- Modify: `internal/runner/runner.go:841-846` (processBead method)

**Step 1: Add the call**

In `internal/runner/runner.go`, in `processBead()`, after `buildPromptForBead` (line 842-845) and before the ATDD check (line 848), add:

```go
	// Pre-compilation check: surface existing build errors in the prompt
	r.runCompilationCheck(ctx, bc)
```

The resulting code should read:

```go
	// Build prompt (with optional scope check)
	if err := r.buildPromptForBead(ctx, bc, iteration); err != nil {
		bc.Result.Error = err
		return bc.Result
	}

	// Pre-compilation check: surface existing build errors in the prompt
	r.runCompilationCheck(ctx, bc)

	// Check if ATDD is active for this bead
	atddActive := bead.IsMethodologyActive(bc.Bead.Labels, "atdd", r.cfg.Methodology.ATDD)
```

**Step 2: Verify compilation and tests**

Run: `go build ./... && go test ./internal/runner/ -count=1`
Expected: exit 0, all tests pass

**Step 3: Commit**

```bash
git add internal/runner/runner.go
git commit -m "Wire runCompilationCheck into processBead before ATDD/build"
```

---

### Task 5: Verify end-to-end and ensure cmdRunnerFn is wired

**Files:**
- Check: `internal/runner/runner.go` (NewRunner and NewRunnerWithDeps constructors)

**Step 1: Verify cmdRunnerFn is set in constructors**

Read `NewRunner` and `NewRunnerWithDeps` to confirm `cmdRunnerFn` is already wired (it should be — it's used by validation). If not wired, add `cmdRunnerFn: defaultCmdRunner` to the constructor.

**Step 2: Run full test suite**

Run: `go test ./... -count=1`
Expected: All pass

**Step 3: Run lint**

Run: `golangci-lint run ./...`
Expected: 0 issues

**Step 4: Final commit if any fixups needed**

```bash
git add -A
git commit -m "Wire compilation check end-to-end"
```
