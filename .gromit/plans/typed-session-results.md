---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T13:01:01-05:00"
id: typed-session-results
source_spec: typed-session-results
---

# Typed Session Results via Generics Implementation Plan

**Goal:** Add typed `Result()` methods to pipeline session wrappers using Go generics, enabling type-safe result retrieval after session completion.

**Architecture:** Three-layer type system — `BaseSession` interface (renamed from `Session`) for lifecycle, `Session[T any]` generic interface for typed results, and non-generic `baseSession` struct for shared subprocess plumbing. Each typed wrapper embeds `*baseSession` and adds its own result fields.

**Tech Stack:** Go 1.26.0 (generics), os/exec for subprocess management, io pipes, goroutines for async stdout reading

**Spec:** `.gromit/specs/typed-session-results.md`

---

## Architecture

**Overview:**
Introduce a three-layer session type system using Go generics. The existing `Session` interface is renamed to `BaseSession`, a new generic `Session[T any]` interface adds typed result retrieval, and a concrete `baseSession` struct handles subprocess lifecycle plumbing shared across all session types.

**Key Components:**
1. **`BaseSession` interface** (renamed from `Session`): Core lifecycle contract — `Events()`, `SendInput()`, `Cancel()`, `Wait()`
2. **`Session[T any]` interface**: Generic interface embedding `BaseSession` + `Result() (T, error)` for type-safe result access
3. **`baseSession` struct**: Non-generic implementation of `BaseSession` — manages `*exec.Cmd`, stdin/stdout pipes, events channel, done channel, cancel func, and postProcess callback
4. **Typed wrappers**: `RefineSession`, `PlanSession`, `ReviewSession`, `ExploreSession` each embed `*baseSession` and add `result T` / `resultErr error` fields with a concrete `Result()` method

**Integration Points:**
- `types.go` — all interface/struct/constructor definitions change here
- New `session.go` — houses the `baseSession` implementation (subprocess lifecycle, goroutine management)
- `session_test.go` — two reference fixes (`Session` → `BaseSession`, manual struct construction)
- `pipeline.go` — no changes needed (returns typed session pointers, not the `Session` interface directly)
- `mocks_test.go` — add compile-time generic interface satisfaction checks

**Data Flow:**
```
newRefineSession(ctx, cmd, postProcessFn)
  → newBaseSession(ctx, cmd, postProcessFn)
    → sets up stdin/stdout pipes on cmd
    → starts cmd
    → launches goroutine: reads stdout → sends EventOutput to events channel
    → sends EventSessionStarted
  → returns &RefineSession{baseSession: bs}

session.Wait()
  → waits on done channel (set when process exits)
  → if exit code 0 and postProcess != nil: calls postProcess()
  → returns error (exit error or postProcess error)

session.Result()
  → if !done: return zero, "session not complete"
  → return result, resultErr
```

**Files to Modify:**
- `internal/pipeline/types.go` — rename interface, add generic interface, restructure wrappers, add constructors and Result() methods
- `internal/pipeline/session_test.go` — fix `Session` → `BaseSession` reference (line 41), fix `RefineSession{Session: baseSession}` construction (line 522)
- `internal/pipeline/mocks_test.go` — add compile-time generic interface checks

**Files to Create:**
- `internal/pipeline/session.go` — `baseSession` struct implementation with full subprocess lifecycle

**Tradeoffs:**
- **Embedding `*baseSession` vs `BaseSession` interface in wrappers**: Using the concrete struct avoids interface indirection and gives wrappers direct access to internal fields (needed for result population). Tighter coupling is acceptable since all wrappers live in the same package.
- **`postProcess` callback vs interface method**: Callback keeps result-production logic localized in each typed constructor rather than requiring a `PostProcess()` interface method that every wrapper must override.

## Test Strategy

**Test Levels:**
1. **Acceptance Tests** (existing `session_test.go`, `//go:build acceptance`): 25 tests covering full `baseSession` lifecycle and typed wrapper behavior using real subprocesses. Making all pass is the definition of done.
2. **Unit Tests** (no build tag): Compile-time interface satisfaction checks verifying generic type relationships hold.
3. **Existing Tests** (`go test ./internal/pipeline/...` without acceptance tag): Must continue passing.

**Key Test Cases (from existing acceptance tests):**
- `baseSession` lifecycle: constructor, Events channel, SessionStarted/Output/SessionEnded/Error events, SendInput, Cancel, Wait, WaitReturnsError, ContextCancellation, ChannelClosedAfterEnd, MultipleSendInput, StdinCloseOnCancel, PipedIOSetup, StdoutReaderGoroutine
- PostProcess: called after success, error propagated
- Typed wrappers: Result() on zero-value returns error, Result() after completion returns initialized result
- Constructors: all four typed constructors return non-nil

**New Compile-Time Checks:**
- `var _ BaseSession = (*baseSession)(nil)`
- `var _ Session[RefineResult] = (*RefineSession)(nil)`
- `var _ Session[PlanResult] = (*PlanSession)(nil)`
- `var _ Session[ReviewResult] = (*ReviewSession)(nil)`
- `var _ Session[ExploreResult] = (*ExploreSession)(nil)`

**Mocking Strategy:** No mocking — acceptance tests use real subprocesses (`echo`, `cat`, `sleep`, `sh -c`).

**Test Organization:**
- `internal/pipeline/session_test.go` — acceptance tests (two minor fixes needed)
- `internal/pipeline/mocks_test.go` — compile-time generic interface checks

## Implementation Tasks

### Task 1: Define session type system with generics

**Files:**
- Modify: `internal/pipeline/types.go`

**What to Do:**
Rename the `Session` interface to `BaseSession`. Add the new `Session[T any]` generic interface that embeds `BaseSession` and adds `Result() (T, error)`. Restructure the four typed wrapper structs (`RefineSession`, `PlanSession`, `ReviewSession`, `ExploreSession`) to embed `*baseSession` (pointer to the concrete struct from Task 2) and add `result` and `resultErr` fields of their corresponding types. Add `Result()` methods on each typed wrapper that return an error if the session isn't complete (checking the `baseSession.completed` flag), or the result/resultErr otherwise. Add constructor functions (`newRefineSession`, `newPlanSession`, `newReviewSession`, `newExploreSession`) that create a `baseSession` via `newBaseSession` and wrap it.

**Acceptance Criteria:**
- `BaseSession` interface has Events, SendInput, Cancel, Wait; old `Session` name is removed
- `Session[T any]` generic interface embeds `BaseSession` and adds `Result() (T, error)`
- Each typed wrapper has `Result()` returning its concrete type and satisfies `Session[T]`

**Dependencies:** Task 2 (baseSession struct definition needed for embedding). In practice, Tasks 1 and 2 will be implemented together as a single bead since they are co-dependent.

### Task 2: Implement baseSession subprocess lifecycle

**Files:**
- Create: `internal/pipeline/session.go`
- Modify: `internal/pipeline/session_test.go`

**What to Do:**
Create `session.go` with the `baseSession` struct holding: `cmd *exec.Cmd`, `stdin io.WriteCloser`, `stdout io.ReadCloser`, `events chan Event`, `done chan struct{}`, `err error`, `cancelFn context.CancelFunc`, `postProcess func() error`, and a `completed bool` flag. Implement `newBaseSession(ctx, cmd, postProcess)` that wraps the context with `WithCancel`, sets up stdin/stdout pipes on the command, starts the process, launches a goroutine to read stdout line-by-line and emit `EventOutput` events, sends `EventSessionStarted` before reading, sends `EventSessionEnded` or `EventError` when done, and closes the events channel. Implement `Events()` returning the receive-only channel, `SendInput()` writing to stdin, `Cancel()` calling the cancel func (which kills the process via context), and `Wait()` blocking on the done channel then invoking `postProcess` on success and setting `completed = true`. Update `session_test.go`: change `var _ Session` to `var _ BaseSession` at line 41, and fix the manual `RefineSession` construction at line 522 to use `baseSession` field name.

**Acceptance Criteria:**
- `baseSession` implements `BaseSession` with full subprocess lifecycle
- `Wait()` invokes postProcess after successful exit; propagates postProcess errors
- All 25 acceptance tests in `session_test.go` pass

**Dependencies:** Task 1 (BaseSession interface definition). Co-dependent — implement together.

**Notes:** The goroutine must close the events channel after sending the terminal event. `Wait()` should be safe to call multiple times (use `sync.Once` or check `done` channel). Context cancellation should kill the subprocess. Stderr from failed processes should be captured for the `EventError` content.

### Task 3: Add compile-time generic interface checks

**Files:**
- Modify: `internal/pipeline/mocks_test.go`

**What to Do:**
Add compile-time interface satisfaction checks: `var _ BaseSession = (*baseSession)(nil)`, `var _ Session[RefineResult] = (*RefineSession)(nil)`, `var _ Session[PlanResult] = (*PlanSession)(nil)`, `var _ Session[ReviewResult] = (*ReviewSession)(nil)`, `var _ Session[ExploreResult] = (*ExploreSession)(nil)`.

**Acceptance Criteria:**
- `go test ./internal/pipeline/...` passes (no acceptance tag needed)
- All five compile-time checks compile

**Dependencies:** Tasks 1 and 2

---

## Notes

- **First generics in codebase**: This introduces `Session[T any]` as the first generic type. Keep it simple — the generic interface is thin (one method) and the concrete implementations are non-generic.
- **Test line references**: session_test.go line 41 (`var _ Session`) and line 522 (`RefineSession{Session: baseSession}`) need updating. These were written as "expected failure" targets before the implementation existed.
- **Goroutine cleanup**: The stdout reader goroutine must reliably close the events channel and signal the done channel, even on context cancellation or process crash. Use `defer` for cleanup.
- **Wait() idempotency**: Multiple `Wait()` calls should return the same error. Use `sync.Once` for the postProcess invocation.
- **No cmd/ impact**: No code in `cmd/` imports the pipeline package yet, so the `Session` → `BaseSession` rename is zero-risk.
