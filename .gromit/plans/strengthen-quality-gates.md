---
id: strengthen-quality-gates
source_spec: strengthen-quality-gates
created: 2026-03-03
decomposed: false
---

# Strengthen Quality Gates Implementation Plan

**Goal:** Add two post-validation quality gates — a regression test gate and a wiring check gate — that catch the two most common defect categories leaking to human QA.

**Architecture:** Two new `pipeline.Stage` implementations slot into the orchestrator between Validate and Epilogue. The wiring gate runs after validation (structural grep-based check, zero LLM cost). The regression gate runs concurrently with review (test execution for packages not already validated). Both feed failures back into the existing retry/escalation loop.

**Tech Stack:** Go, existing pipeline/config/events packages, `go/ast` or grep for symbol analysis

**Spec:** `.gromit/specs/strengthen-quality-gates.md`

---

## Architecture

**Overview:**
Two new `pipeline.Stage` implementations — wiring gate and regression gate — integrate into the existing orchestrator pipeline. The wiring gate is structural (diff parsing + grep), the regression gate runs `go test`. Both reuse the existing retry loop for failure recovery.

**Key Components:**

1. **`internal/pipeline/qualitygate/wiring/` package**: Implements `pipeline.Stage`. Parses git diff to extract newly-exported Go symbols, then checks whether each is referenced from at least one file outside its defining file. Returns `Block` with failure messages for unwired symbols, `Proceed` when all symbols are wired or gate is disabled.

2. **`internal/pipeline/qualitygate/regression/` package**: Implements `pipeline.Stage`. Runs the project test suite excluding packages already covered by the validation stage (using `TouchedPackages` from Input). Returns `Block` with failing test output, `Proceed` when all tests pass or gate is disabled/skipped.

3. **`internal/config/config_types.go` extension**: New `QualityGatesConfig` struct with `Regression` and `Wiring` sub-configs, added to the root `Config`.

4. **`internal/runner/orchestrator.go` modification**: After validation passes, run wiring gate. If wiring gate fails, re-enter retry loop. After wiring gate passes, run regression gate and review concurrently via goroutines. If regression gate fails, re-enter retry loop.

5. **`internal/failurephase/failurephase.go` extension**: New `WiringGate` and `RegressionGate` constants.

6. **`internal/pipeline/stage.go` extension**: New `WiringFailures []string` field on `Output` for wiring gate failure messages (parallel to `ValidationFailures`).

**Integration Points:**
- Orchestrator: after validation passes (line ~759) and before review (line ~812)
- Constructor: wire new stages into `OrchestratorConfig`
- Config loading: `SetDefaults`, `NormalizeNilFields`, `Validate` for new section
- Existing `TouchedPackages` on Input provides package exclusion data for regression gate

**Data Flow:**
```
Build → Validate (produces TouchedPackages, ValidationFailures)
  → Wiring Gate (reads git diff, greps for references → Block/Proceed)
    → [Regression Gate + Review] in parallel (goroutines)
      Regression Gate: runs `go test ./...` excluding TouchedPackages → Block/Proceed
      Review: existing non-blocking LLM review → always Proceed
  → Epilogue
```

**Files to Modify:**
- `internal/runner/orchestrator.go` — Add wiring gate call, parallelize regression+review
- `internal/runner/constructor.go` — Wire new stages into OrchestratorConfig
- `internal/config/config_types.go` — Add `QualityGatesConfig` struct and field
- `internal/config/defaults.go` — Set defaults (both enabled, regression command)
- `internal/config/normalize.go` — Normalize nil fields
- `internal/pipeline/stage.go` — Add `WiringFailures` to Output
- `internal/failurephase/failurephase.go` — Add `WiringGate`, `RegressionGate` constants

**Files to Create:**
- `internal/pipeline/qualitygate/wiring/wiring.go` — Wiring gate stage
- `internal/pipeline/qualitygate/wiring/wiring_test.go` — Tests
- `internal/pipeline/qualitygate/wiring/symbols.go` — Symbol extraction from diff
- `internal/pipeline/qualitygate/wiring/symbols_test.go` — Tests
- `internal/pipeline/qualitygate/regression/regression.go` — Regression gate stage
- `internal/pipeline/qualitygate/regression/regression_test.go` — Tests

**Tradeoffs:**
- **Grep over AST for wiring**: Grep is simpler and sufficient for "is this symbol name referenced elsewhere." AST would be more precise but adds complexity for marginal benefit. False positives (symbol name in comment) are rare and harmless (they pass, not fail).
- **Package-level exclusion for regression**: We exclude entire packages from TouchedPackages rather than individual tests. Clean heuristic that avoids redundant work.
- **New failure phases**: Distinct `WiringGate` and `RegressionGate` phases keep failure reporting clear vs. overloading `Validation`.
- **Parallel regression+review**: Localized concurrency (just these two stages) using standard Go goroutine patterns. Review is already non-blocking, so regression gate failure is the only outcome that matters.

---

## Test Strategy

**Test Levels:**

1. **Unit Tests**: Symbol extraction parsing, reference checking logic, package exclusion computation, config defaults/normalization
2. **Integration Tests**: Stage-level Run() with real/mocked dependencies, orchestrator retry flow with gate failures
3. **Manual Testing**: Full gromit loop with intentionally unwired symbols and regression-causing changes

**Key Test Cases:**
- Symbol extraction: parses `func NewFoo()`, `type Bar struct`, `func (b *Bar) Method()`, exported struct fields from unified diff
- Exclusions: skips `_test.go` exports, `// wiring:deferred` symbols, interface method declarations
- Reference found: symbol in `pkg/a/a.go` referenced in `pkg/b/b.go` → passes
- Reference missing: symbol only in defining file → fails with message
- Wiring stage: `Proceed` when all wired, `Block` when unwired, `Proceed` when disabled
- Package exclusion: correctly computes test targets excluding TouchedPackages
- Regression passes: all non-touched packages pass → `Proceed`
- Regression fails: non-touched package fails → `Block` with output
- Skip label: `skip-regression-check` → `Proceed` without running
- Gate disabled: config `enabled: false` → `Proceed`
- Config defaults: both gates enabled, regression command = `go test ./...`
- Orchestrator: wiring failure triggers retry, regression failure triggers retry, parallel execution of regression+review

**Mocking Strategy:**
- `GitDiffFn` function field (existing pattern from Review) for controlled diff output
- Command runner interface for regression gate to avoid real `go test` in unit tests
- Real grep/file ops for wiring reference checks against test fixture directories

**Test Organization:**
- `internal/pipeline/qualitygate/wiring/wiring_test.go` — Stage tests
- `internal/pipeline/qualitygate/wiring/symbols_test.go` — Symbol extraction unit tests
- `internal/pipeline/qualitygate/regression/regression_test.go` — Stage tests
- Orchestrator integration tests in existing `internal/runner/orchestrator_test.go`

---

## Implementation Tasks

### Task 1: Config and Failure Phases

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/defaults.go`
- Modify: `internal/config/normalize.go`
- Modify: `internal/failurephase/failurephase.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `QualityGatesConfig` to the config system and new failure phase constants. The config struct has `Regression` (Enabled bool, Command string) and `Wiring` (Enabled bool) sub-structs. Add `QualityGates QualityGatesConfig` field to root Config with `yaml:"quality_gates"` tag. Set defaults (both enabled, command = `go test ./...`). Add `WiringGate` and `RegressionGate` failure phase constants.

**Acceptance Criteria:**
- `QualityGatesConfig` loads from YAML with correct defaults when omitted
- `NormalizeNilFields` handles the new config section
- `WiringGate` and `RegressionGate` failure phase constants exist

**Dependencies:** None

### Task 2: Pipeline Stage Extensions

**Files:**
- Modify: `internal/pipeline/stage.go`
- Modify: `internal/runner/orchestrator.go` (OrchestratorConfig struct only)

**What to Do:**
Add `WiringFailures []string` to `pipeline.Output` for wiring gate failure messages. Add `WiringGate pipeline.Stage` and `RegressionGate pipeline.Stage` fields to `OrchestratorConfig`. Do not yet modify the Run loop — just the struct definitions.

**Acceptance Criteria:**
- `pipeline.Output` has `WiringFailures []string` field
- `OrchestratorConfig` has `WiringGate` and `RegressionGate` stage fields

**Dependencies:** Task 1

### Task 3: Wiring Gate — Symbol Extraction

**Files:**
- Create: `internal/pipeline/qualitygate/wiring/symbols.go`
- Create: `internal/pipeline/qualitygate/wiring/symbols_test.go`

**What to Do:**
Implement symbol extraction from unified diff output. Parse added lines (prefixed with `+`) to identify newly-exported Go symbols: functions (`func ExportedName`), types (`type ExportedName`), methods (`func (r *Receiver) ExportedName`), and exported struct fields. Return a list of `Symbol{Name, File, Line}` structs. Handle exclusions: skip symbols in `_test.go` files, skip symbols annotated with `// wiring:deferred` on the preceding line, skip interface method declarations (inside `interface {}` blocks).

**Acceptance Criteria:**
- Correctly extracts exported functions, types, methods, struct fields from diff
- Excludes `_test.go` symbols, `// wiring:deferred` symbols, interface methods
- Handles multi-line function signatures, embedded types, method receivers

**Dependencies:** None

### Task 4: Wiring Gate — Reference Checker and Stage

**Files:**
- Create: `internal/pipeline/qualitygate/wiring/wiring.go`
- Create: `internal/pipeline/qualitygate/wiring/wiring_test.go`

**What to Do:**
Implement the reference checker: for each extracted symbol, grep the codebase for references outside the symbol's defining file. Implement the `pipeline.Stage` interface: get git diff, extract symbols, check references, return `Proceed` if all wired or gate disabled, `Block` with `WiringFailures` listing unwired symbols. Use a `GitDiffFn` function field (same pattern as Review stage) for testability.

**Acceptance Criteria:**
- Returns `Proceed` when all new symbols are referenced externally
- Returns `Block` with actionable failure messages for unwired symbols
- Returns `Proceed` when gate is disabled via config
- GitDiffFn is injectable for testing

**Dependencies:** Task 2, Task 3

### Task 5: Regression Gate — Package Exclusion and Stage

**Files:**
- Create: `internal/pipeline/qualitygate/regression/regression.go`
- Create: `internal/pipeline/qualitygate/regression/regression_test.go`

**What to Do:**
Implement the regression gate stage. Compute test targets by listing all Go packages and excluding those in `Input.TouchedPackages`. Run the configured test command (from `QualityGatesConfig.Regression.Command`) against the remaining packages. Return `Proceed` if all pass or gate is disabled/skipped, `Block` with test output if any fail. Check `skip-regression-check` label on the bead to bypass. Use a command runner interface for testability.

**Acceptance Criteria:**
- Correctly excludes TouchedPackages from test targets
- Returns `Proceed` on all tests passing, `Block` with output on failure
- Respects `skip-regression-check` bead label
- Returns `Proceed` when gate disabled via config

**Dependencies:** Task 1, Task 2

### Task 6: Orchestrator Integration

**Files:**
- Modify: `internal/runner/orchestrator.go` (Run loop)
- Modify: `internal/runner/constructor.go`

**What to Do:**
Wire the new stages into the orchestrator. After validation passes and before review: run the wiring gate. If wiring gate returns `Block`, treat as a gate failure — set failure phase to `WiringGate`, feed `WiringFailures` back as `ValidationFailures` for the retry loop, and re-enter the Build→Validate→WiringGate retry. After wiring gate passes: run regression gate and review concurrently using goroutines. If regression gate returns `Block`, treat as a gate failure — set failure phase to `RegressionGate`, feed test output back for retry. In constructor.go, create the concrete stage instances and wire them into `OrchestratorConfig`.

**Acceptance Criteria:**
- Wiring gate runs after validation passes, before review
- Wiring gate failure triggers Build→Validate→WiringGate retry with failure context
- Regression gate and review run concurrently (goroutines)
- Regression gate failure triggers retry with test output as context
- Constructor creates and wires both stages

**Dependencies:** Task 4, Task 5

### Task 7: Orchestrator Integration Tests

**Files:**
- Modify: `internal/runner/orchestrator_test.go`

**What to Do:**
Add integration tests for the new gate behavior in the orchestrator:
1. Wiring gate failure triggers retry loop (mock wiring gate returns Block, then Proceed)
2. Regression gate failure triggers retry loop (mock regression gate returns Block, then Proceed)
3. Both gates disabled → pipeline proceeds without them
4. `skip-regression-check` label bypasses regression gate
5. Regression gate and review run concurrently (verify via mock timing or call order)

**Acceptance Criteria:**
- Wiring gate retry path verified with mock stages
- Regression gate retry path verified with mock stages
- Disabled/skipped gates verified
- Concurrent regression+review execution verified

**Dependencies:** Task 6

---

## Notes

- The orchestrator's existing validation retry loop (lines 667-693) is the model for how gate failures should feed back. The wiring gate retry should follow the same pattern: rebuild with failure context, re-validate, re-check wiring gate.
- The regression gate runs concurrently with review, but if regression fails, the bead re-enters the retry loop. On retry, regression will run again after the next successful validation+wiring pass. This means subsequent retries include the full pipeline including regression — the fix is verified end-to-end.
- `TouchedPackages` is already populated on `Input` by the orchestrator (line 66 of stage.go). The regression gate consumes this directly.
- The wiring gate's grep-based approach may produce false negatives for symbols referenced only via reflection or code generation. This is acceptable — the goal is catching the common case of "wrote a function but forgot to call it," not exhaustive reachability analysis.
- Both gates are no-ops when their config is disabled, returning `Proceed` immediately with zero cost.
