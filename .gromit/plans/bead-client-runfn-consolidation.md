---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T18:30:46Z"
id: bead-client-runfn-consolidation
source_spec: bead-client-runfn-consolidation
---

# Bead Client RunFn Consolidation Implementation Plan

**Goal:** Consolidate `internal/bead.Client` command injection to a single exported `RunFn` hook and migrate in-package tests to that canonical hook without changing CLI behavior.

**Architecture:** Remove the private `runFn` field from `bead.Client` and simplify `Client.run` to a single injection branch (`RunFn`) plus existing subprocess fallback path (`exec.Command`, `Dir`, and wrapped stderr on exit errors).

**Tech Stack:** Go (`testing`, `os/exec`), existing `internal/bead` test suite.

**Spec:** `.gromit/specs/bead-client-runfn-consolidation.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Consolidate command injection in `internal/bead.Client` to one exported hook (`RunFn`) and remove dual-hook precedence logic from `run()`. This preserves runtime behavior while eliminating ambiguity in internal test setup.

**Key Components:**
1. **`internal/bead.Client` struct**: Remove unexported `runFn` so there is one authoritative injection field.
2. **`(*Client).run(args ...string)`**: Keep exact fallback subprocess behavior, but only check `RunFn` before fallback.
3. **`internal/bead` tests**: Replace struct literals using `runFn:` with `RunFn:` and keep existing command/parse assertions unchanged.

**Integration Points:**
- Existing methods (`Ready`, `ListWithLabel`, `Close`, `AddComment`, `Create`, etc.) already route through `c.run(...)`; they inherit consolidation with no call-site logic changes.
- Cross-package tests in `cmd/gromit` already use `RunFn` and should remain unchanged.
- Acceptance/unit tests under `internal/bead` are the primary migration surface.

**Data Flow:**
- Method calls (for example `Ready()`) build args and call `c.run(args...)`.
- `run()` now performs:
  1. `RunFn != nil` -> invoke injected function and return.
  2. Else execute `bd` subprocess with existing `binary` + `Dir` behavior.
  3. On exit error, preserve stderr-wrapping behavior.
- Output parsing and business logic remain unchanged.

**Files to Modify:**
- `internal/bead/bead.go` - remove `runFn` field and simplify `run()` injection logic.
- `internal/bead/bead_test.go` - migrate `runFn:` test injection to `RunFn:`.
- `internal/bead/ready_limit_unit_test.go` - migrate `runFn:` to `RunFn:`.
- `internal/bead/ready_with_label_test.go` - migrate `runFn:` to `RunFn:` and message text referencing `runFn`.
- `internal/bead/list_with_label_test.go` - migrate `runFn:` to `RunFn:`.
- `internal/bead/bd_subprocess_optimization_acceptance_test.go` - migrate `runFn:` to `RunFn:`.
- `internal/bead/ready_limit_test.go` - align expected-failure comments/usages with canonical hook where applicable.

**Files to Create:**
- None.

**Tradeoffs:**
- **Keep exported `RunFn` as canonical hook**: chosen to preserve existing external test usage and cross-package injectability.
- **Remove `runFn` entirely instead of retaining alias**: chosen to remove precedence ambiguity and prevent future drift between hooks.
- **No command-path refactor beyond injection consolidation**: chosen to minimize risk and keep subprocess semantics identical.

**Checkpoint status:** Approved implicitly via plan-generation request in this session; any adjustment requests can be incorporated before decomposition.

## Test Strategy

## Test Strategy Proposal

**Test Levels:**
1. **Unit Tests:** Validate all migrated in-package tests still exercise command construction, validation, and parse behavior through `RunFn` injection.
2. **Integration/Acceptance Tests:** Run `internal/bead` acceptance-tagged tests that depend on injection behavior to ensure no regression in fallback assumptions.
3. **Manual/Static Verification:** Grep-based checks to ensure no residual `runFn` field/usages in `internal/bead` source and tests.

**Key Test Cases:**
- `Client.run` invokes `RunFn` when set.
- `Client.run` falls back to subprocess when `RunFn` is nil.
- Exit-error stderr wrapping behavior remains unchanged.
- Existing behaviors in `Ready`, `ReadyWithLabel`, `ListWithLabel`, `AddComment`, and `Create` continue to pass with `RunFn`-based injection.
- Cross-package tests constructing `bead.Client{RunFn: ...}` compile and pass unchanged.

**Mocking Strategy:**
- Continue using function injection (`RunFn`) for command stubbing in unit tests.
- Keep real subprocess execution only in tests that intentionally verify subprocess path.
- Avoid introducing new fake abstractions; use current lightweight closure-based mocks.

**Coverage Goals:**
- Critical path: single-hook dispatch logic in `Client.run`.
- Regression guard: no dual-hook fallback remains.
- Edge behavior: nil hook fallback, stderr-wrapped exit errors, `Dir` passthrough unaffected.

**Test Organization:**
- Keep tests in existing files; only update struct-field names and any stale test wording.
- Use targeted package runs first (`go test ./internal/bead/...`), then broader validation as needed.

**Checkpoint status:** Approved implicitly via plan-generation request in this session; add/remove specific regression cases before decomposition if desired.

## Implementation Tasks

### Task 1: Consolidate Client Injection Hook

**Files:**
- Modify: `internal/bead/bead.go`
- Test: `internal/bead/bead_test.go` (existing tests touching `Client.run` behavior)

**What to Do:**
Remove `runFn` from `Client`, update `run()` to check only `RunFn`, and retain exact subprocess fallback behavior (`exec.Command`, optional `Dir`, exit stderr wrapping).

**Acceptance Criteria:**
- `Client` defines only `RunFn func(args ...string) (string, error)` as command injection hook.
- `(*Client).run` has no `runFn` branch and checks only `RunFn` before subprocess fallback.
- Subprocess error/Dir behavior is unchanged from pre-consolidation behavior.

**Dependencies:**
- None.

**Notes:**
Keep this task narrowly scoped to structural consolidation; do not alter command args or parsing flows.

### Task 2: Migrate Internal Bead Tests to RunFn

**Files:**
- Modify: `internal/bead/bead_test.go`
- Modify: `internal/bead/ready_limit_unit_test.go`
- Modify: `internal/bead/ready_with_label_test.go`
- Modify: `internal/bead/list_with_label_test.go`
- Modify: `internal/bead/bd_subprocess_optimization_acceptance_test.go`
- Modify: `internal/bead/ready_limit_test.go`

**What to Do:**
Replace all `runFn:` struct literal initializations with `RunFn:` and update any assertion/comment text that names `runFn` so tests align with canonical API terminology.

**Acceptance Criteria:**
- Repository search `rg -n "runFn:" internal/bead` returns zero matches.
- Tests continue to verify the same behaviors as before (no semantic assertion weakening).
- Any expected-failure comments reflect the final API state and no longer contradict the code.

**Dependencies:**
- Task 1.

**Notes:**
Prefer mechanical field-name migration first, then fix message/comment drift in a second pass to reduce mistakes.

### Task 3: Regression Validation and Acceptance Checks

**Files:**
- No code changes required unless regressions are found.

**What to Do:**
Run targeted tests and static checks to confirm consolidation is complete and behavior preserved.

**Acceptance Criteria:**
- `go test ./internal/bead/...` passes.
- Relevant cross-package tests relying on `bead.Client{RunFn: ...}` compile/pass (for example in `cmd/gromit`).
- `rg -n "\brunFn\b" internal/bead` returns zero production/test references after migration.

**Dependencies:**
- Task 2.

**Notes:**
If failures appear, treat them as migration regressions and capture follow-up beads only for unrelated breakage.

---

## Notes

- Scope is intentionally narrow: API clarity and deterministic injection behavior only.
- Do not change `RunFn` signature or command/business semantics.
- Keep decomposition-friendly granularity: one core logic task, one migration task, one validation task.
