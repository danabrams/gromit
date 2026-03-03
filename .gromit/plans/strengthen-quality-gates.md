---
id: strengthen-quality-gates
source_spec: strengthen-quality-gates
created: 2026-03-03
decomposed: false
---

# Strengthen Quality Gates Implementation Plan

**Goal:** Add two new quality gates — regression testing and wiring verification — that catch the two most common defect categories leaking to human QA.

**Architecture:** Two independent `pipeline.Stage` implementations under `internal/pipeline/qualitygate/`. The wiring gate runs sequentially after validation (structural analysis, zero token cost). The regression gate runs concurrently with review (test execution). Both feed failures into the existing retry/escalation loop.

**Tech Stack:** Go, `go/ast` or regex-based symbol extraction, existing `CommandRunner` interface for test execution

**Spec:** `.gromit/specs/strengthen-quality-gates.md`

---

## Architecture

**Overview:**
Two new pipeline stages inserted into the orchestrator's success path after validation passes. The wiring gate is structural (grep/regex-based) and the regression gate is test-based.

**Key Components:**

1. **`internal/pipeline/qualitygate/wiring/`**: Extracts newly-exported Go symbols from git diff, checks for references outside the defining file. Zero token cost.
2. **`internal/pipeline/qualitygate/regression/`**: Runs project test suite for packages not already covered by validation's fast commands.
3. **`QualityGatesConfig`** in config: Top-level `quality_gates` YAML key with `regression` and `wiring` sub-configs.

**Integration Points:**
- Both gates implement `pipeline.Stage`
- New `WiringGate` and `RegressionGate` fields on `OrchestratorConfig`
- `TouchedPackages` from validation tells regression gate which packages to exclude
- Wiring gate uses injected `GitDiffFn` (same pattern as review stage)
- Failures set `ValidationFailures` on `pipeline.Input`, triggering existing retry loop
- `skip-regression-check` label checked against `bead.Labels`

**Pipeline Order:**
```
Build → Validate → Wiring Gate → [Regression Gate + Review] (parallel) → Epilogue
```

**Tradeoffs:**
- Regex on diff output rather than full AST walk: simpler, sufficient for Go export convention (capital letter), avoids parsing entire codebase
- Concurrent regression+review via goroutines in orchestrator rather than composite stage: keeps Stage interface clean
- "Everything minus validated" rather than snapshot comparison: simpler, matches spec decision #6

## Test Strategy

**Unit Tests:**
- Symbol extraction from unified diff format (functions, types, methods, consts, vars)
- Reference checking logic (found/not found, exclusions for `_test.go`, `// wiring:deferred`, interface satisfaction)
- Package exclusion logic for regression gate
- Config defaults and validation

**Integration Tests:**
- Orchestrator-level tests with stubbed stages verifying gate execution order
- Concurrent regression+review execution
- Retry loop re-entry on gate failure
- Label-based skip behavior

**Mocking:**
- `CommandRunner` for regression gate test execution
- `GitDiffFn` for wiring gate diff access
- File reader/grep function for reference checking
- Stub `pipeline.Stage` implementations in orchestrator tests

**Coverage Goals:**
- Symbol extraction accuracy (critical path)
- Package exclusion correctness
- Concurrent execution correctness
- No false positives on existing well-wired, non-regressing code

---

## Implementation Tasks

### Task 1: Add config types and failure phase constants

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config_normalize.go`
- Modify: `internal/failurephase/failurephase.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `QualityGatesConfig` struct with `Regression` (`RegressionGateConfig`: `Enabled bool`, `Command string`) and `Wiring` (`WiringGateConfig`: `Enabled bool`) sub-configs. Add `QualityGates QualityGatesConfig` field to the top-level `Config` struct with YAML tag `quality_gates`. Set defaults (both enabled, regression command = `go test ./...`). Add nil-field normalization. Add `WiringGate = "wiring_gate"` and `RegressionGate = "regression_gate"` failure phase constants.

**Acceptance Criteria:**
- `QualityGatesConfig` parses from YAML with correct defaults
- Both gates default to enabled
- Regression command defaults to `go test ./...`
- New failure phase constants exist

**Dependencies:** None

### Task 2: Implement wiring gate symbol extraction

**Files:**
- Create: `internal/pipeline/qualitygate/wiring/symbols.go`
- Create: `internal/pipeline/qualitygate/wiring/symbols_test.go`

**What to Do:**
Implement `ExtractNewExports(diff string) []ExportedSymbol` that parses unified diff output and returns newly-added exported Go symbols. Each `ExportedSymbol` has `Name`, `File`, `Kind` (func/type/method/const/var), and `HasDeferredComment bool`. Only lines starting with `+` (not `++`) in the diff are considered. Only symbols starting with an uppercase letter are exported. Track current file from `--- a/` / `+++ b/` diff headers. Detect `// wiring:deferred` comments on the same or preceding line.

**Acceptance Criteria:**
- Correctly extracts exported functions (`func FooBar(`), types (`type FooBar struct/interface`), methods (`func (r *Receiver) FooBar(`), and const/var declarations
- Ignores unexported symbols, modified (non-`+`) lines, and `_test.go` files
- Detects `// wiring:deferred` annotation

**Dependencies:** None

### Task 3: Implement wiring gate reference checker and stage

**Files:**
- Create: `internal/pipeline/qualitygate/wiring/wiring.go`
- Create: `internal/pipeline/qualitygate/wiring/wiring_test.go`

**What to Do:**
Implement the `Wiring` stage (`pipeline.Stage`). Inject a `GitDiffFn` and a `ReferenceChecker` interface (`CheckReference(ctx context.Context, symbol, definingFile string) (bool, error)`). In `Run`: get diff, extract new exports via `ExtractNewExports`, filter out `_test.go` and `// wiring:deferred` symbols, check each remaining symbol for references outside its defining file. If any are unwired, return `Block` with `ValidationFailures` containing an actionable message listing the unwired symbols. Provide a default `GrepReferenceChecker` that uses `grep -r` (or Go file walking) to find references.

**Acceptance Criteria:**
- Returns `Proceed` when all new exports are referenced
- Returns `Block` with clear failure messages when exports are unreferenced
- Excludes `_test.go` exports and `// wiring:deferred` symbols
- Returns `Proceed` when no new exports exist (no-op)

**Dependencies:** Task 2

### Task 4: Implement regression gate stage

**Files:**
- Create: `internal/pipeline/qualitygate/regression/regression.go`
- Create: `internal/pipeline/qualitygate/regression/regression_test.go`

**What to Do:**
Implement the `Regression` stage (`pipeline.Stage`). Inject a `CommandRunner` interface (same as validate stage). In `Run`: compute untested packages by taking the configured regression command (default `go test ./...`) and excluding packages already covered by validation's `fast_commands` (available from `config.ScopeGoTestCommands` inverse logic). If no untested packages remain, return `Proceed`. Otherwise run the test command scoped to untested packages. On failure, return `Block` with test output as `ValidationFailures`. Check for `skip-regression-check` label on bead and return `Proceed` immediately if present.

**Acceptance Criteria:**
- Runs tests only for packages not covered by validation
- Returns `Proceed` when all remaining tests pass
- Returns `Block` with test output on failure
- Skips when `skip-regression-check` label is present
- Returns `Proceed` when validation already covered all packages

**Dependencies:** Task 1

### Task 5: Wire gates into orchestrator and constructor

**Files:**
- Modify: `internal/runner/orchestrator.go`
- Modify: `internal/runner/constructor.go`
- Test: `internal/runner/orchestrator_test.go` (or new `orchestrator_quality_gates_test.go`)

**What to Do:**
Add `WiringGate pipeline.Stage` and `RegressionGate pipeline.Stage` fields to `OrchestratorConfig`. In the orchestrator's success path (after validation passes, before review): run `WiringGate` if non-nil. If it returns `Block`, treat as a failure (set failure phase to `failurephase.WiringGate`, emit failure event, run epilogue on failure path, continue). After wiring gate passes: run `RegressionGate` concurrently with `Review` using goroutines. If regression returns `Block`, treat as failure (set failure phase to `failurephase.RegressionGate`). In constructor: construct wiring and regression stages when their respective configs are enabled, wire `GitDiffFn` and `CommandRunner` into stages, set them on `OrchestratorConfig`.

**Acceptance Criteria:**
- Wiring gate runs after validation, before review
- Regression gate runs concurrently with review
- Wiring gate failure triggers retry loop with wiring failure context
- Regression gate failure triggers retry loop with test failure context
- Gates are nil (skipped) when disabled in config
- Existing tests continue to pass (no behavioral change when gates are nil)

**Dependencies:** Tasks 1, 3, 4

### Task 6: End-to-end orchestrator integration tests

**Files:**
- Create or modify: `internal/runner/orchestrator_quality_gates_test.go`

**What to Do:**
Write integration-level tests using stub stages that verify the full gate flow:
1. Wiring gate fails → retry loop entered with wiring failure context
2. Regression gate fails → retry loop entered after parallel stage with regression failure context
3. Both gates pass → normal success path
4. Gates disabled → existing behavior unchanged
5. `skip-regression-check` label → regression gate skipped
6. Wiring gate fails then passes on retry → bead succeeds
7. Regression + review run concurrently (verify both are called)

**Acceptance Criteria:**
- All scenarios pass with correct stage execution order
- Failure phases correctly set in iteration logs
- Concurrent execution of regression+review verified

**Dependencies:** Task 5

---

## Notes

- The wiring gate's reference checker should be efficient — it runs on every successful validation. Grep-based search scoped to Go files should be fast enough for typical project sizes.
- The regression gate's package exclusion logic inverts `ScopeGoTestCommands` — instead of scoping to touched packages, it scopes to everything *except* touched packages. May need a new helper `ExcludeGoTestPackages` or similar.
- Interface satisfaction detection for the wiring gate is the hardest exclusion to implement precisely. A pragmatic approach: if a new method's receiver type implements an interface defined elsewhere, exclude it. This can be approximated by checking if the method signature matches any interface method in the codebase, but for v1, it's acceptable to rely on `// wiring:deferred` as an escape hatch and refine later.
- The concurrent regression+review execution should use `sync.WaitGroup` or `errgroup.Group` to ensure both complete before proceeding. Review errors are already discarded; regression errors trigger failure.
