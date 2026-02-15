---
id: typed-session-results
source_ideas: []
created: 2026-02-12
---

# Typed Session Results via Generics

## Specification

Pipeline session wrappers (RefineSession, PlanSession, ReviewSession, ExploreSession) gain typed `Result()` methods that return their corresponding result types (RefineResult, PlanResult, ReviewResult, ExploreResult). This is achieved through Go generics, introducing the first generic types in the codebase.

The architecture has three layers:

1. **`BaseSession` interface** — the existing non-generic session interface, renamed from `Session`. Defines the core session lifecycle: `Events()`, `SendInput()`, `Cancel()`, `Wait()`.

2. **`Session[T any]` interface** — a new generic interface that embeds `BaseSession` and adds `Result() (T, error)`. Consumers that need typed results use this interface.

3. **`baseSession` struct** — a concrete, non-generic struct that implements `BaseSession`. Holds shared session plumbing (cmd, stdin, stdout, events channel, done channel, error, cancel func). Each typed session wrapper embeds this struct.

4. **Typed session wrappers** — `RefineSession`, `PlanSession`, `ReviewSession`, `ExploreSession` each embed `baseSession` and add their own `result` and `resultErr` fields of the appropriate type. Each implements `Result()` with its concrete return type, satisfying `Session[T]` for its corresponding result type.

**Result population** works via a post-process callback. The `baseSession` accepts a `postProcess func() error` at construction time. When `Wait()` completes successfully, it invokes the callback. Each typed session's constructor provides a phase-specific callback that parses session output and populates the result fields.

**Result retrieval** is non-blocking and explicit. Calling `Result()` before `Wait()` completes returns a zero value and an error (`"session not complete"`). The expected caller pattern is:

```go
err := session.Wait()
result, err := session.Result()
```

`DecomposeResult` is excluded from this work — `Pipeline.Decompose()` already returns `*DecomposeResult` directly since it is non-interactive.

## Acceptance Criteria

- `BaseSession` interface exists with the four existing methods (Events, SendInput, Cancel, Wait) and the old `Session` interface name is removed
- `Session[T any]` generic interface exists, embeds `BaseSession`, and adds `Result() (T, error)`
- `baseSession` struct implements `BaseSession` with shared session lifecycle plumbing (cmd, stdin, stdout, events channel, done channel, cancel func, postProcess callback)
- `RefineSession` embeds `baseSession`, has a `Result() (RefineResult, error)` method, and satisfies `Session[RefineResult]`
- `PlanSession` embeds `baseSession`, has a `Result() (PlanResult, error)` method, and satisfies `Session[PlanResult]`
- `ReviewSession` embeds `baseSession`, has a `Result() (ReviewResult, error)` method, and satisfies `Session[ReviewResult]`
- `ExploreSession` embeds `baseSession`, has a `Result() (ExploreResult, error)` method, and satisfies `Session[ExploreResult]`
- `Result()` returns an error when called before `Wait()` completes
- `Wait()` invokes the `postProcess` callback after the subprocess exits successfully
- Constructors exist for baseSession and each typed session (`newBaseSession`, `newRefineSession`, `newPlanSession`, `newReviewSession`, `newExploreSession`)
- Existing acceptance tests in `session_test.go` pass

## Decisions

1. **Use generics (Approach B: generic interface + non-generic baseSession)** — The `Session[T]` generic interface provides type safety for consumers, while the `baseSession` struct stays non-generic to keep shared plumbing simple. Each typed wrapper embeds `baseSession` and adds its own typed result fields. This avoids the limitations of type aliases (Approach A) which would prevent adding session-specific methods later.

2. **Rename `Session` to `BaseSession`, use `Session[T]` for the generic interface** — This resolves the naming conflict cleanly. The non-generic `BaseSession` is available for code that doesn't care about result types. The rename is low-impact since no code in `cmd/` imports the pipeline package yet.

3. **Post-process callback for result population** — Each typed session's constructor receives a phase-specific `postProcess func() error` callback. When `Wait()` completes, it invokes the callback, which parses output and populates result fields. This keeps result-production logic self-contained within each session rather than pushing it to the orchestrator.

4. **Non-blocking `Result()` with explicit error** — `Result()` returns immediately. If the session hasn't finished, it returns a zero value and `"session not complete"` error. Callers must call `Wait()` first. This matches Go conventions and avoids surprising implicit blocking.

5. **Exclude DecomposeResult** — `Pipeline.Decompose()` returns `*DecomposeResult` directly because decomposition is non-interactive. It doesn't use a session wrapper, so it's out of scope.

## Research & Context

### Current State

- **`internal/pipeline/types.go`** — Defines `Session` interface (to be renamed `BaseSession`), all four typed session wrappers (currently just embedding `Session` with no additional fields/methods), all Result types and their constructors, and all Input types.
- **`internal/pipeline/pipeline.go`** — Defines `Pipeline` struct, `Deps`/`Paths`, 8 dependency interfaces, and stub workflow methods. Workflow methods return typed session pointers (e.g., `*RefineSession`) but are not yet implemented.
- **`internal/pipeline/session_test.go`** — Acceptance tests (build tag `acceptance`) that expect `baseSession`, typed constructors, and `Result()` methods. These tests are written to fail against the current code and will serve as the implementation target.
- **`internal/pipeline/helpers.go`** — Utility functions (`ListMarkdownFiles`, `DiffFiles`, `ExtractSpecTitle`, `WriteTempPrompt`) used by pipeline workflows.
- **`internal/pipeline/mocks_test.go`** — Compile-time interface satisfaction checks for all 8 dependency interfaces.
- **Go version is 1.26.0** — Generics fully supported. This will be the first use of generics in the codebase.
- **No `cmd/` integration yet** — All five command handlers work inline without using the pipeline package, so the `Session` → `BaseSession` rename has zero downstream impact today.
