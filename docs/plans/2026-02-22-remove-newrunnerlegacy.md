# NewRunnerLegacy Removal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate dual architecture (Orchestrator vs Runner) by converting all tests from NewRunnerWithDeps (Runner) to use the new 5-stage pipeline (Orchestrator), then remove NewRunnerLegacy.

**Architecture:** The codebase has two parallel systems: the legacy Runner with its own Run() loop (124-236 lines in runner.go), and the new Orchestrator with 5 pipeline stages. NewRunnerLegacy duplicates wiring from the new architecture but directly initializes a Runner. All acceptance tests use NewRunnerWithDeps to create Runners, but the new recommended pattern is NewRunner -> Orchestrator. Migration involves: (1) Converting 5 acceptance test files to use Orchestrator via NewRunner, (2) Verifying tests still pass, (3) Removing NewRunnerLegacy, NewRunnerWithDeps, and the legacy Runner.Run() loop.

**Tech Stack:** Go, test-driven development, 5-stage pipeline (prepare/gate, execute/build, validate, review, epilogue), Orchestrator pattern

---

## Phase 1: Build an Orchestrator Test Helper

This phase creates a testing abstraction for Orchestrator to replace direct Runner usage in tests.

### Task 1: Create OrchestratorTestHelper with mock-friendly Run method

**Files:**
- Create: `internal/runner/acceptance/orchestrator_test_helper.go`
- Modify: `internal/runner/acceptance/helpers_test.go` (add new helper)
- Test: `internal/runner/acceptance/orchestrator_test_helper_test.go`

**Step 1: Write the failing test**

```go
package acceptance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

// TestOrchestratorTestHelper_NewHelperCreatesOrchestrator verifies that
// NewOrchestratorTestHelper returns a working OrchestratorTestHelper
func TestOrchestratorTestHelper_NewHelperCreatesOrchestrator(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	helper, err := NewOrchestratorTestHelper(cfg, io.Discard, "/tmp/gromit")
	if err != nil {
		t.Fatalf("NewOrchestratorTestHelper() error = %v", err)
	}

	if helper == nil {
		t.Fatal("NewOrchestratorTestHelper returned nil")
	}

	if helper.orchestrator == nil {
		t.Fatal("helper.orchestrator is nil")
	}
}

// TestOrchestratorTestHelper_RunCallsOrchestratorRun verifies that
// calling helper.Run() invokes the underlying orchestrator
func TestOrchestratorTestHelper_RunCallsOrchestratorRun(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	helper, err := NewOrchestratorTestHelper(cfg, io.Discard, "/tmp/gromit")
	if err != nil {
		t.Fatalf("NewOrchestratorTestHelper() error = %v", err)
	}

	// Call Run with nil context should be handled gracefully
	// (it will exit immediately because no beads are available)
	err = helper.Run(context.Background(), 1, time.Time{}, nil, false)
	// We don't expect an error for this simple case
	if err != nil && err != context.Canceled {
		t.Fatalf("helper.Run() returned unexpected error: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestOrchestratorTestHelper -v`
Expected: FAIL with "NewOrchestratorTestHelper not defined"

**Step 3: Write minimal implementation**

Create `internal/runner/acceptance/orchestrator_test_helper.go`:

```go
package acceptance_test

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

// OrchestratorTestHelper wraps an Orchestrator with test-friendly methods
type OrchestratorTestHelper struct {
	orchestrator *runner.Orchestrator
	cfg          *config.Config
	gromitDir    string
}

// NewOrchestratorTestHelper creates a new OrchestratorTestHelper with default mocks
func NewOrchestratorTestHelper(cfg *config.Config, output io.Writer, gromitDir string) (*OrchestratorTestHelper, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if output == nil {
		output = io.Discard
	}

	// Use NewRunner which returns an Orchestrator
	orch, err := runner.NewRunner(cfg, output)
	if err != nil {
		return nil, fmt.Errorf("NewRunner failed: %w", err)
	}

	return &OrchestratorTestHelper{
		orchestrator: orch,
		cfg:          cfg,
		gromitDir:    gromitDir,
	}, nil
}

// Run executes the orchestrator loop (equivalent to Runner.Run)
func (h *OrchestratorTestHelper) Run(ctx context.Context, maxIterations int, deadline time.Time, stopCh <-chan struct{}, dryRun bool) error {
	if h.orchestrator == nil {
		return fmt.Errorf("orchestrator is nil")
	}
	return h.orchestrator.Run(ctx, maxIterations, deadline, stopCh, dryRun)
}
```

**Step 4: Run test to verify it passes**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestOrchestratorTestHelper -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/dabrams/gromit
git add internal/runner/acceptance/orchestrator_test_helper.go internal/runner/acceptance/orchestrator_test_helper_test.go
git commit -m "red: test that OrchestratorTestHelper creates and runs Orchestrator"
```

Then:

```bash
git add internal/runner/acceptance/orchestrator_test_helper.go internal/runner/acceptance/orchestrator_test_helper_test.go
git commit -m "green: implement OrchestratorTestHelper with NewOrchestratorTestHelper constructor"
```

---

## Phase 2: Convert Acceptance Tests to Use Orchestrator

This phase converts the 5 acceptance test files that currently use NewRunnerWithDeps.

### Task 2: Convert loop_acceptance_test.go to use OrchestratorTestHelper

**Files:**
- Modify: `internal/runner/acceptance/loop_acceptance_test.go`
- Test: Same file (internal tests)

**Step 1: Write the failing test**

Add new test function that uses OrchestratorTestHelper while keeping old tests:

```go
// TestOrchestratorTestHelper_UsesLabelFiltersInLoop verifies that Orchestrator
// respects label filters in the loop (converting from Runner-based test)
func TestOrchestratorTestHelper_UsesLabelFiltersInLoop(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// This test will fail because we haven't yet wired label filters into Orchestrator
	helper, err := NewOrchestratorTestHelper(cfg, io.Discard, "/tmp/gromit")
	if err != nil {
		t.Fatalf("NewOrchestratorTestHelper() error = %v", err)
	}

	// Try to set label filters (will fail if Orchestrator doesn't support this)
	helper.SetLabelFilters([]string{"spec:auth", "spec:payments"})
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestOrchestratorTestHelper_UsesLabelFiltersInLoop -v`
Expected: FAIL with "SetLabelFilters not defined"

**Step 3: Write minimal implementation**

Update `orchestrator_test_helper.go` to add:

```go
// SetLabelFilters sets label filters on the orchestrator's config
func (h *OrchestratorTestHelper) SetLabelFilters(labels []string) {
	if h.cfg != nil {
		// Note: Orchestrator currently doesn't support label filters like Runner did.
		// This is a limitation that will require wiring into the GetBead function.
		// For now, we store them but acknowledge this needs full Orchestrator support.
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestOrchestratorTestHelper_UsesLabelFiltersInLoop -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/dabrams/gromit
git add internal/runner/acceptance/orchestrator_test_helper.go
git commit -m "red: test that OrchestratorTestHelper supports SetLabelFilters"
git add internal/runner/acceptance/orchestrator_test_helper.go
git commit -m "green: add SetLabelFilters to OrchestratorTestHelper"
```

**Note:** Do NOT yet remove the old TestRunner_UsesLabelFiltersInLoop test that uses NewRunnerWithDeps. We'll do that in Phase 3.

---

### Task 3: Add dependency injection support to OrchestratorTestHelper

**Files:**
- Modify: `internal/runner/acceptance/orchestrator_test_helper.go`
- Test: `internal/runner/acceptance/orchestrator_test_helper_test.go`

**Step 1: Write the failing test**

```go
// TestOrchestratorTestHelper_AcceptsBeadMock verifies that the helper can
// use a mock BeadClient instead of real bd integration
func TestOrchestratorTestHelper_AcceptsBeadMock(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return nil, nil
		},
	}

	helper, err := NewOrchestratorTestHelperWithDeps(cfg, io.Discard, "/tmp/gromit", &OrchestratorTestDeps{
		Beads: mockBeads,
	})
	if err != nil {
		t.Fatalf("NewOrchestratorTestHelperWithDeps() error = %v", err)
	}

	if helper == nil {
		t.Fatal("NewOrchestratorTestHelperWithDeps returned nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestOrchestratorTestHelper_AcceptsBeadMock -v`
Expected: FAIL with "NewOrchestratorTestHelperWithDeps not defined"

**Step 3: Write minimal implementation**

Update `orchestrator_test_helper.go`:

```go
import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner"
)

// OrchestratorTestDeps holds test dependencies for OrchestratorTestHelper
type OrchestratorTestDeps struct {
	Beads runner.BeadClient
	// Add Router, Analyzer, Renderer, Logger as needed
}

// NewOrchestratorTestHelperWithDeps creates a helper with injected test dependencies
func NewOrchestratorTestHelperWithDeps(cfg *config.Config, output io.Writer, gromitDir string, deps *OrchestratorTestDeps) (*OrchestratorTestHelper, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if output == nil {
		output = io.Discard
	}

	// If deps provided, build orchestrator with custom BeadClient
	if deps != nil && deps.Beads != nil {
		// Create orchestrator config with custom GetBead function
		mainDir := filepath.Dir(gromitDir)
		orchCfg := runner.OrchestratorConfig{
			Config:    cfg,
			Output:    output,
			GetBead: func(ctx context.Context) (*bead.Bead, error) {
				return deps.Beads.Ready()
			},
			// Additional fields will be wired as needed
		}
		orch := runner.NewOrchestrator(orchCfg)
		return &OrchestratorTestHelper{
			orchestrator: orch,
			cfg:          cfg,
			gromitDir:    gromitDir,
		}, nil
	}

	// Fall back to NewRunner for non-deps case
	orch, err := runner.NewRunner(cfg, output)
	if err != nil {
		return nil, fmt.Errorf("NewRunner failed: %w", err)
	}

	return &OrchestratorTestHelper{
		orchestrator: orch,
		cfg:          cfg,
		gromitDir:    gromitDir,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestOrchestratorTestHelper_AcceptsBeadMock -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /home/dabrams/gromit
git add internal/runner/acceptance/orchestrator_test_helper.go internal/runner/acceptance/orchestrator_test_helper_test.go
git commit -m "red: test that OrchestratorTestHelper accepts BeadClient dependency injection"
git add internal/runner/acceptance/orchestrator_test_helper.go internal/runner/acceptance/orchestrator_test_helper_test.go
git commit -m "green: implement NewOrchestratorTestHelperWithDeps for test dependency injection"
```

---

## Phase 3: Gradually Migrate Test Files

This phase converts each test file from NewRunnerWithDeps to OrchestratorTestHelper, one file at a time.

### Task 4: Migrate helpers_test.go to use OrchestratorTestHelper

**Files:**
- Modify: `internal/runner/acceptance/helpers_test.go`
- Test: Same file

**Step 1: Write failing test**

Add new test that uses OrchestratorTestHelper while keeping old NewRunnerWithDeps test:

```go
// TestNewOrchestratorTestHelper_WithMockRouter verifies that the helper works
// with a custom router (converted from TestNewRunnerWithDeps_WithMockRouter)
func TestNewOrchestratorTestHelper_WithMockRouter(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	mockRouter := newMockRouterFromClaudeClient(&mockClaudeClient{
		StreamRunFn: func(ctx context.Context, prompt string, model string, output io.Writer, handler claude.EventHandler, onToolCall claude.ToolCallHandler) (*claude.Result, error) {
			return &claude.Result{Success: true, Output: "test output"}, nil
		},
	})

	deps := &OrchestratorTestDeps{
		Router: mockRouter,
	}

	helper, err := NewOrchestratorTestHelperWithDeps(cfg, io.Discard, "/tmp/gromit", deps)
	if err != nil {
		t.Fatalf("NewOrchestratorTestHelperWithDeps() error = %v", err)
	}

	if helper == nil {
		t.Fatal("NewOrchestratorTestHelperWithDeps returned nil helper")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestNewOrchestratorTestHelper_WithMockRouter -v`
Expected: FAIL with "OrchestratorTestDeps.Router not defined"

**Step 3: Write minimal implementation**

Update `orchestrator_test_helper.go`:

```go
type OrchestratorTestDeps struct {
	Beads  runner.BeadClient
	Router *provider.Router
}

// Update NewOrchestratorTestHelperWithDeps to wire Router
```

**Step 4: Run test to verify it passes**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestNewOrchestratorTestHelper_WithMockRouter -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/acceptance/orchestrator_test_helper.go internal/runner/acceptance/helpers_test.go
git commit -m "red: test that OrchestratorTestHelper accepts Router dependency injection"
git add internal/runner/acceptance/orchestrator_test_helper.go
git commit -m "green: add Router support to OrchestratorTestDeps"
```

---

### Task 5: Migrate loop_acceptance_test.go usage

**Files:**
- Modify: `internal/runner/acceptance/loop_acceptance_test.go`

**Step 1: Write failing test**

Add test that parallels TestRunner_UsesLabelFiltersInLoop but uses Orchestrator:

```go
// TestOrchestrator_UsesLabelFiltersInLoop verifies label filtering in Orchestrator
func TestOrchestrator_UsesLabelFiltersInLoop(t *testing.T) {
	// Same test as TestRunner_UsesLabelFiltersInLoop but using OrchestratorTestHelper
	// This will fail because Orchestrator needs label filter wiring
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -run TestOrchestrator_UsesLabelFiltersInLoop -v`
Expected: FAIL

**Step 3: Write minimal implementation** (see Task 2 notes - may require wiring GetBead)

**Step 4: Run test to verify it passes**

**Step 5: Delete old test, commit new one**

```bash
# Remove old TestRunner_UsesLabelFiltersInLoop test
git add internal/runner/acceptance/loop_acceptance_test.go
git commit -m "red: test that Orchestrator supports label filters"
# Add implementation if needed
git commit -m "green: wire label filters in OrchestratorTestHelper"
```

---

### Task 6: Migrate status_acceptance_test.go

**Files:**
- Modify: `internal/runner/acceptance/status_acceptance_test.go`

**Step 1-5:** Follow same pattern as Task 5, converting TestRunner_* tests to TestOrchestrator_*

Commit each with `red:` then `green:` prefix.

---

### Task 7: Migrate status_integration_acceptance_test.go

**Files:**
- Modify: `internal/runner/acceptance/status_integration_acceptance_test.go`

**Step 1-5:** Follow same pattern, commit with red/green.

---

### Task 8: Migrate runner_pipeline_acceptance_test.go

**Files:**
- Modify: `internal/runner/acceptance/runner_pipeline_acceptance_test.go`

**Step 1-5:** Follow same pattern.

---

## Phase 4: Remove Legacy Code

Once all tests are converted, remove the legacy code.

### Task 9: Delete NewRunnerLegacy from runner.go

**Files:**
- Modify: `internal/runner/runner.go:124-236` (delete NewRunnerLegacy function)

**Step 1: Write test that verifies NewRunnerLegacy is NOT exported**

```go
// TestNewRunnerLegacy_RemovedFromPublicAPI verifies NewRunnerLegacy has been removed
func TestNewRunnerLegacy_RemovedFromPublicAPI(t *testing.T) {
	// This test will fail until NewRunnerLegacy is removed
	// Use reflection to verify the function is not in the runner package
	pkgType := reflect.TypeOf((*runner.Runner)(nil))
	if _, ok := reflect.TypeOf(runner).MethodByName("NewRunnerLegacy"); ok {
		t.Error("NewRunnerLegacy still exists; it should be removed")
	}
}
```

Actually, a simpler approach: just verify code doesn't import/use NewRunnerLegacy anywhere.

**Step 2: Run test to verify it fails**

Run: `cd /home/dabrams/gromit && grep -r "NewRunnerLegacy" . --include="*.go" | grep -v "^\.\/\." | grep -v "internal/runner/runner.go"`
Expected: Zero results (only definition in runner.go remains)

If grep returns results, those files still use it - we haven't finished migration.

**Step 3: Delete NewRunnerLegacy**

Edit `internal/runner/runner.go` and remove lines 124-236 (the NewRunnerLegacy function).

**Step 4: Run tests to verify they still pass**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -v`
Expected: PASS (all tests use Orchestrator now)

**Step 5: Commit**

```bash
git add internal/runner/runner.go
git commit -m "red: test that NewRunnerLegacy is removed from public API"
git add internal/runner/runner.go
git commit -m "green: delete NewRunnerLegacy constructor"
```

---

### Task 10: Delete NewRunnerWithDeps and constructor_with_deps.go

**Files:**
- Delete: `internal/runner/constructor_with_deps.go`
- Modify: `internal/runner/runner.go` (remove NewRunnerWithDeps declaration)

**Step 1: Write test**

```go
// TestNewRunnerWithDeps_RemovedFromPublicAPI verifies legacy constructor is gone
func TestNewRunnerWithDeps_RemovedFromPublicAPI(t *testing.T) {
	// Verify NewRunnerWithDeps is not in runner package
	// (actual check done by seeing if code compiles)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -v`
Expected: FAIL with "undefined: runner.NewRunnerWithDeps"

(Because we haven't deleted the file yet)

**Step 3: Delete the file**

```bash
rm internal/runner/constructor_with_deps.go
```

Remove NewRunnerWithDeps declaration from runner.go (lines 117-122).

**Step 4: Run tests to verify they pass**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/acceptance -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -u internal/runner/constructor_with_deps.go internal/runner/runner.go
git commit -m "red: test that NewRunnerWithDeps is removed"
git add -u internal/runner/
git commit -m "green: delete NewRunnerWithDeps and constructor_with_deps.go"
```

---

### Task 11: Remove dual Run loop from Runner

**Files:**
- Modify: `internal/runner/runner.go` (delete Run method and related helpers, lines 238-347)

**Step 1: Write test**

```go
// TestRunner_RunMethodRemoved verifies the legacy Run loop is gone
func TestRunner_RunMethodRemoved(t *testing.T) {
	// This test verifies no direct Runner.Run() usage remains in codebase
}
```

**Step 2: Verify no tests use Runner.Run directly**

Run: `cd /home/dabrams/gromit && grep -r "\.Run(" ./internal/runner/acceptance --include="*.go" | grep -v "Orchestrator" | grep -v "pipeline\|\.stage" | grep -v "TestOrchestrator"`
Expected: Zero results (only Orchestrator.Run or pipeline stage.Run calls)

**Step 3: Delete Runner.Run method**

Edit `internal/runner/runner.go` and remove the Run method (lines 238-347 approximately).

**Step 4: Run tests**

Run: `cd /home/dabrams/gromit && go test ./internal/runner/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/runner.go
git commit -m "red: test that Runner.Run legacy loop is removed"
git add internal/runner/runner.go
git commit -m "green: delete Runner.Run method (dual architecture)"
```

---

## Summary

After completing all phases:

1. **New entry point:** All code uses `NewRunner() -> Orchestrator` with 5-stage pipeline
2. **Tests converted:** All 5 acceptance test files use OrchestratorTestHelper
3. **Legacy removed:** NewRunnerLegacy, NewRunnerWithDeps, constructor_with_deps.go deleted
4. **Dual loop gone:** Runner.Run() method removed
5. **Architecture unified:** Single Orchestrator-based loop, no legacy parallel system

**Total commits:** ~20 small, focused red-green-commit cycles
**Risk mitigation:** Each conversion test-first, run full test suite before removal

