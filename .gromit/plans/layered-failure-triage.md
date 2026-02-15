---
id: layered-failure-triage
source_spec: layered-failure-triage
created: 2026-02-15
decomposed: true
---

# Layered Failure Triage Implementation Plan

**Goal:** Add a fast, programmatic triage step that classifies build failures into four layers before deciding the response path, so only code-level failures invoke the LLM analyzer.

**Architecture:** A `Triage()` function in the `escalation` package classifies failures using `provider.Result.FailureCategory`, stderr pattern matching, and structural checks. `ExecuteWithRetry` calls it between `showPartialProgress` and `AnalyzeAndHandleFailure`, short-circuiting for non-code layers.

**Tech Stack:** Go, regexp, existing escalation/provider/runtypes packages

**Spec:** `.gromit/specs/layered-failure-triage.md`

---

## Architecture

**Key Components:**

1. **`escalation/triage.go`** — `FailureLayer` constants, `TriageResult` type, `Triage()` function with waterfall detection: provider_transport → environment → orchestration → code.

2. **`escalation/handler.go`** — `ExecuteWithRetry` calls `Triage()` after partial progress, dispatches per-layer response before `AnalyzeAndHandleFailure()`.

3. **`execution/invoker.go` + `escalation/handler.go` InvocationResult** — Carry `provider.Result` through so triage can read `FailureCategory` and `Stderr`.

4. **`runtypes/types.go` + `logger/logger.go`** — `FailureLayer` and `FailureSubCat` observability fields.

**Data Flow:**
```
provider.StreamRun → provider.Result (FailureCategory, Stderr, Output)
  → execution.InvocationResult.ProviderResult
  → escalation.InvocationResult.ProviderResult
  → Triage() reads FailureCategory + Stderr/Output → TriageResult
  → ExecuteWithRetry dispatches per layer
  → IterationResult.FailureLayer + FailureSubCat → IterationLog
```

**Integration Points:**
- `ExecuteWithRetry` calls `Triage()` after `showPartialProgress()`, before `AnalyzeAndHandleFailure()`
- `makeInvokeFn` populates `ProviderResult` on escalation `InvocationResult`
- `writeIterationLog` writes the new fields
- `IsScopeTooLarge` stays in `makeInvokeFn` (already handled before triage runs)

**Tradeoffs:**
- Carry full `provider.Result` rather than individual fields — forward-compatible with gromit-o5i4
- Triage in escalation package — co-locates control-flow decisions
- `IsScopeTooLarge` stays in `makeInvokeFn` — scope-too-large never reaches the triage point since it's handled upstream

## Test Strategy

**Unit Tests** (`triage_test.go`): Table-driven tests for each layer with representative `provider.Result` inputs. Every sub-category tested. Waterfall ordering verified. Edge cases: nil ProviderResult, empty strings, FailureCategory "other".

**Integration Tests** (`handler_test.go`): Tests verifying `ExecuteWithRetry` short-circuits for non-code layers. Key assertion: mock analyzer tracks whether `Analyze()` was called — must NOT be called for transport/environment/orchestration layers.

**Mocking:** Mock `FailureAnalyzer` to detect calls. Mock `InvokeFn` to return crafted results. Reuse existing `newTestBeadContext()` and `newTestConfig()` helpers.

## Implementation Tasks

### Task 1: Carry provider.Result through InvocationResult

**Files:**
- Modify: `internal/runner/execution/invoker.go`
- Modify: `internal/runner/escalation/handler.go`
- Modify: `internal/runner/callbacks.go`

**What to Do:**
Add `ProviderResult *provider.Result` field to `execution.InvocationResult` (invoker.go:20). Populate it from the existing `providerResult` variable in `Execute()` (invoker.go:173). Add the same field to `escalation.InvocationResult` (handler.go:35). In `makeInvokeFn` (callbacks.go), propagate the `ProviderResult` from `execution.InvocationResult` through to `escalation.InvocationResult`. The `executeClaudeInvocation` helper returns a `claude.Result` — extend it to also return the `provider.Result` from the execution result so callbacks.go can access it. Update all `escalation.InvocationResult` literal constructions in callbacks.go (the stall/timeout/success paths) to include `ProviderResult` where available.

**Acceptance Criteria:**
- `execution.InvocationResult` has `ProviderResult *provider.Result` populated after `Execute()`
- `escalation.InvocationResult` has `ProviderResult *provider.Result` populated in `makeInvokeFn`
- All existing tests pass unchanged

**Dependencies:** None

**Notes:**
`executeClaudeInvocation` currently returns `(*claude.Result, *logger.StreamStats, bool, error)`. Either add a 5th return value for `*provider.Result`, or return the full `execution.InvocationResult` instead of destructuring. The latter is cleaner but touches more code. Either approach works — the key is that the `provider.Result` reaches `makeInvokeFn`.

### Task 2: Add triage types and Triage() function

**Files:**
- Create: `internal/runner/escalation/triage.go`
- Create: `internal/runner/escalation/triage_test.go`

**What to Do:**
Create `triage.go` with: `FailureLayer` string type and four constants (`LayerProviderTransport`, `LayerEnvironment`, `LayerOrchestration`, `LayerCode`). `TriageResult` struct with `Layer`, `SubCategory`, `Detail`, `Retryable` fields. `Triage()` function that takes `*InvocationResult` and `*runtypes.BeadContext` and returns `*TriageResult`.

Triage waterfall logic:
1. **Provider transport**: Check `ProviderResult.FailureCategory` — `transport_disconnect` → sub `disconnect` (retryable), `rate_limited` → sub `rate_limit` (retryable), `auth` → sub `auth` (not retryable).
2. **Environment**: Compile four regexes, check against `ProviderResult.Stderr` (fall back to `ProviderResult.Output` when stderr empty): `exec: .+: executable file not found` → `missing_tool`, `go: go\.mod requires go >=` → `version_mismatch`, `no space left on device` → `resource_exhausted`, `permission denied` → `permission`. All not retryable.
3. **Orchestration**: Check `bc.BuildPrompt == ""` → `bad_prompt`. Check `bc.Bead.Description == ""` (no acceptance criteria check — `Bead` struct has no AC field) → `bad_bead`. Not retryable.
4. **Code**: Default fallthrough. Retryable.

If `ProviderResult` is nil, fall through to `code` layer (safe default).

Create `triage_test.go` with table-driven tests covering every sub-category, the waterfall ordering, stderr-to-output fallback, nil ProviderResult, FailureCategory "other", and empty strings.

**Acceptance Criteria:**
- `Triage()` classifies each failure category/pattern into the correct layer and sub-category
- Table-driven tests cover all 10+ sub-categories with representative inputs
- Nil `ProviderResult` safely falls through to `code` layer
- Environment patterns check Stderr first, then Output

**Dependencies:** Task 1 (needs `ProviderResult` on `InvocationResult`)

### Task 3: Wire triage into ExecuteWithRetry with per-layer response handling

**Files:**
- Modify: `internal/runner/escalation/handler.go`
- Modify: `internal/runner/escalation/handler_test.go`

**What to Do:**
In `ExecuteWithRetry`, after `showPartialProgress` and the context cancellation check (line 354), insert a call to `Triage(invResult, bc)` where `invResult` is the `InvocationResult` returned by `invokeFn`. Note: currently `ExecuteWithRetry` only has `claudeResult` at that point — the full `InvocationResult` is consumed earlier. Refactor to keep the `InvocationResult` in scope through the triage point.

Based on `TriageResult.Layer`:

- **`provider_transport`**: If retryable (`disconnect`, `rate_limit`), increment `bc.RetriesThisModel++` and `bc.TotalRetriesThisBead++`, set `bc.Result.FailureLayer` and `FailureSubCat`, then `continue` (retry). If not retryable (`auth`), set `bc.Result.Error` with actionable message, set layer fields, return `false`.
- **`environment`**: Set `bc.Result.Error` with actionable message (e.g., "Environment error: `go` not found in PATH"), set layer fields, return `false`.
- **`orchestration`**: For `bad_prompt`/`bad_bead`, set error and return `false`. (Scope-too-large is handled upstream in makeInvokeFn.)
- **`code`**: Fall through to existing `AnalyzeAndHandleFailure()`.

Record `TriageResult.Layer` and `SubCategory` on `bc.Result.FailureLayer` and `bc.Result.FailureSubCat` for all layers.

Add integration tests in `handler_test.go`:
- Transport disconnect → retries, analyzer NOT called
- Auth failure → stops immediately, analyzer NOT called
- Environment (missing tool) → stops with actionable error, analyzer NOT called
- Code failure → analyzer IS called (backward compatibility)

Use mock analyzer that records whether `Analyze()` was called.

**Acceptance Criteria:**
- `ExecuteWithRetry` calls `Triage()` before `AnalyzeAndHandleFailure()`
- Transport retryable failures retry using existing budget (no analyzer call)
- Auth failures fail immediately with clear message
- Environment failures fail fast with actionable error messages
- Only code layer invokes the LLM analyzer
- `bc.Result.FailureLayer` and `FailureSubCat` populated for all layers

**Dependencies:** Task 1 (ProviderResult on InvocationResult), Task 2 (Triage function)

### Task 4: Add observability fields to IterationResult and IterationLog

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
Add `FailureLayer string` and `FailureSubCat string` to `IterationResult` in runtypes/types.go (after the existing diagnostic fields block). Add `FailureLayer string` with json tag `"failure_layer,omitempty"` and `FailureSubCat string` with json tag `"failure_sub_cat,omitempty"` to `IterationLog` in logger/logger.go (after the existing diagnostic fields). Wire the fields through in `writeIterationLog` in runner.go — copy from `result.FailureLayer` and `result.FailureSubCat` to the log struct.

**Acceptance Criteria:**
- `IterationResult` has `FailureLayer` and `FailureSubCat` string fields
- `IterationLog` has matching fields with `omitempty` JSON tags
- `writeIterationLog` copies both fields from result to log
- Existing tests pass (fields are additive, zero-value is empty string)

**Dependencies:** None (can run in parallel with Tasks 1-2, fields just need to exist before Task 3 populates them)

---

## Notes

- The `Bead` struct has no `AcceptanceCriteria` field — it only has `Description`. The spec's `bad_bead` check ("no description and no acceptance criteria") simplifies to checking `Description == ""` only.
- `IsScopeTooLarge` is handled in `makeInvokeFn` before `ExecuteWithRetry` sees the failure, so the orchestration layer's `scope_too_large` sub-category won't trigger in practice from triage. It could be added later if scope detection moves into the escalation handler.
- Transport retries count against `RetriesThisModel` and `TotalRetriesThisBead` per the spec — same budget as code-level retries.
- Environment regex patterns should be compiled once (package-level `var` with `regexp.MustCompile`), not per-call.
- The existing `makeInvokeFn` in callbacks.go currently constructs error-path `InvocationResult` literals (lines 40, 46, 51) that return nil Result — these should set `ProviderResult: nil` (the zero value), which triage handles safely by falling through to `code`.
