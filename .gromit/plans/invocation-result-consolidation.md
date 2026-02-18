---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T11:50:01Z"
id: invocation-result-consolidation
source_spec: invocation-result-consolidation
---

# Invocation Result Consolidation Implementation Plan

**Goal:** Consolidate two near-identical `InvocationResult` structs into a single type in `runtypes/` and simplify `executeClaudeInvocation` from a 5-value return to `(*runtypes.InvocationResult, error)`.

**Architecture:** Union of `execution.InvocationResult` and `escalation.InvocationResult` fields into a single `runtypes.InvocationResult` struct, then thread it through the invoker → process → callbacks → escalation chain without unpacking/repacking.

**Tech Stack:** Go

**Spec:** `.gromit/specs/invocation-result-consolidation.md`

---

## Architecture

**Overview:**
Two sibling packages (`execution`, `escalation`) each define their own `InvocationResult` to avoid cross-importing each other. The `process.go` layer unpacks the execution version into 5 return values, and `callbacks.go` repacks several into the escalation version. Moving the struct to `runtypes/` (which both already import) eliminates the duplication and the unpack/repack dance.

**Unified Type (runtypes/types.go):**
```go
type InvocationResult struct {
    Result         *claude.Result
    Stats          *logger.StreamStats
    StallFired     bool
    ModelName      string
    ProviderName   string
    ProviderResult *provider.Result
    TimeoutType    string // "stall", "invocation", "bead", ""
}
```

This is the union of both structs. No field conflicts exist.

**Import Safety:**
`runtypes` currently imports `bead` and `prompt`. Adding `claude`, `logger`, and `provider` is safe — none import `runtypes`.

**Data Flow (before):**
```
invoker.Execute() → *execution.InvocationResult
  → process.executeClaudeInvocation() unpacks to 5 values
    → callbacks.makeInvokeFn() repacks into *escalation.InvocationResult
      → escalation.ExecuteWithRetry() consumes
```

**Data Flow (after):**
```
invoker.Execute() → *runtypes.InvocationResult
  → process.executeClaudeInvocation() forwards directly
    → callbacks.makeInvokeFn() sets TimeoutType on same struct
      → escalation.ExecuteWithRetry() consumes
```

## Test Strategy

- **No new tests** — pure mechanical refactoring with no behavioral changes
- **All existing tests updated in-place** — 5-value destructuring replaced with struct field access
- **Validation:** `go build ./...` and `go test ./internal/runner/...` must pass

**Key existing tests (signature updates only):**
- `TestStallInterrupt` — `invResult.StallFired`, `invResult.Result`, `invResult.Stats`
- `TestExecuteClaudeInvocation_ReturnsProviderResult` — `invResult.ProviderResult`
- `TestExecuteClaudeInvocation_CapturesRateLimitRecoveryMs` — `invResult.Stats`
- `TestExecuteClaudeInvocation_ZeroRecoveryMsWhenNoRateLimit` — err only
- `TestExecuteClaudeInvocationSetsBuildProvider` — all values discarded

## Implementation Tasks

### Task 1: Add unified InvocationResult to runtypes

**Files:**
- Modify: `internal/runner/runtypes/types.go`

**What to Do:**
Add the `InvocationResult` struct with all 7 fields (union of execution + escalation). Add imports for `claude`, `logger`, and `provider` packages.

**Acceptance Criteria:**
- `InvocationResult` struct exists in `runtypes` with all 7 fields
- New imports compile cleanly (`go build ./internal/runner/runtypes/...`)

**Dependencies:** None

### Task 2: Switch execution.Invoker to runtypes.InvocationResult

**Files:**
- Modify: `internal/runner/execution/invoker.go`

**What to Do:**
Delete the local `InvocationResult` struct definition. Change `Execute()` return type from `*InvocationResult` to `*runtypes.InvocationResult`. Update the construction site to use `&runtypes.InvocationResult{...}`.

**Acceptance Criteria:**
- No `InvocationResult` struct in `execution` package
- `Execute()` returns `*runtypes.InvocationResult`
- `go build ./internal/runner/execution/...` passes

**Dependencies:** Task 1

### Task 3: Switch escalation.Handler to runtypes.InvocationResult

**Files:**
- Modify: `internal/runner/escalation/handler.go`

**What to Do:**
Delete the local `InvocationResult` struct and its "mirroring" comment. Change `InvokeFn` type signature to return `*runtypes.InvocationResult`. Update all field accesses throughout the handler (TimeoutType, StallFired, Result, ProviderResult).

**Acceptance Criteria:**
- No `InvocationResult` struct in `escalation` package
- `InvokeFn` uses `*runtypes.InvocationResult`
- `go build ./internal/runner/escalation/...` passes

**Dependencies:** Task 1

### Task 4: Simplify executeClaudeInvocation and update callbacks

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/callbacks.go`

**What to Do:**
Change `executeClaudeInvocation` return from `(*claude.Result, *logger.StreamStats, *provider.Result, bool, error)` to `(*runtypes.InvocationResult, error)`. The body simplifies to forwarding `invResult` directly. In `callbacks.go`, update `makeInvokeFn` to access fields via `invResult.Result`, `invResult.Stats`, etc. Set `invResult.TimeoutType` directly on the shared struct instead of constructing new escalation structs.

**Acceptance Criteria:**
- `executeClaudeInvocation` returns `(*runtypes.InvocationResult, error)`
- `makeInvokeFn` no longer constructs `escalation.InvocationResult` — uses `runtypes.InvocationResult` throughout
- `go build ./internal/runner/...` passes

**Dependencies:** Tasks 2, 3

### Task 5: Update test files for new signature

**Files:**
- Modify: `internal/runner/process_test.go`
- Modify: `internal/runner/execute_claude_invocation_provider_result_test.go`
- Modify: `internal/runner/rate_limit_recovery_logging_test.go`
- Modify: `internal/runner/cross_review_routing_test.go`

**What to Do:**
Replace all 5-value destructuring of `executeClaudeInvocation` with 2-value `(invResult, err)` and struct field access:
- `TestStallInterrupt`: `invResult.Result`, `invResult.Stats`, `invResult.StallFired`
- `TestExecuteClaudeInvocation_ReturnsProviderResult`: `invResult.Result`, `invResult.Stats`, `invResult.ProviderResult`, `invResult.StallFired`
- `TestExecuteClaudeInvocation_CapturesRateLimitRecoveryMs`: `invResult.Stats`
- `TestExecuteClaudeInvocation_ZeroRecoveryMsWhenNoRateLimit`: err only (discard invResult)
- `TestExecuteClaudeInvocationSetsBuildProvider`: discard both (already discards all)

**Acceptance Criteria:**
- No 5-value destructuring of `executeClaudeInvocation` remains
- `go test ./internal/runner/...` passes
- All 5 test functions pass with unchanged behavior

**Dependencies:** Task 4

### Task 6: Final validation

**Files:** None (verification only)

**What to Do:**
Run `go build ./...` and `go test ./internal/runner/...` to confirm everything compiles and passes. Verify no remaining references to `execution.InvocationResult` or `escalation.InvocationResult` struct definitions.

**Acceptance Criteria:**
- `go build ./...` passes
- `go test ./internal/runner/...` passes
- No `InvocationResult` struct definitions remain in `execution` or `escalation` packages

**Dependencies:** Task 5

---

## Notes

- This refactoring also closes bead `gromit-2s2q` ("Carry provider.Result through InvocationResult") since `ProviderResult` is already present on both existing structs and will be carried through the unified type.
- The `escalation/handler_test.go` and `escalation/invocation_result_provider_result_test.go` may need `InvocationResult` literal updates — check during Task 3 if they construct the escalation type directly.
- Tasks 2 and 3 are independent and can be worked in parallel.
- Task 4 must wait for both 2 and 3 since `callbacks.go` bridges both packages.
