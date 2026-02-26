---
id: invocation-result-consolidation
source_ideas: []
created: 2026-02-18
epic: codebase-health
---

# Invocation Result Consolidation

## Specification

`executeClaudeInvocation` returns five values `(*claude.Result, *logger.StreamStats, *provider.Result, bool, error)` by unpacking `execution.InvocationResult`, only for its caller in `callbacks.go` to destructure them and repack several into `escalation.InvocationResult`. Two near-identical `InvocationResult` structs exist across sibling packages to avoid a cross-import. This spec consolidates both into a single type in `runtypes/` and simplifies the return signature.

### Unified Type

Add to `runtypes/types.go`:

```go
// InvocationResult captures the outcome of a single Claude invocation.
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

This is the union of `execution.InvocationResult` (has `ModelName`, `ProviderName`) and `escalation.InvocationResult` (has `TimeoutType`). No field conflicts.

### Import Safety

`runtypes` already imports `bead` and `prompt`. Adding `claude`, `logger`, and `provider` is safe — none of those packages import `runtypes`.

### Changes

**`internal/runner/runtypes/types.go`** — Add `InvocationResult` struct. Add imports for `claude`, `logger`, `provider`.

**`internal/runner/execution/invoker.go`** — Delete local `InvocationResult` struct. Change `Execute()` return type to `*runtypes.InvocationResult`. Update all construction sites to use the runtypes type.

**`internal/runner/escalation/handler.go`** — Delete local `InvocationResult` struct and its "mirroring" comment. Change `InvokeFn` signature to return `*runtypes.InvocationResult`. Update all references.

**`internal/runner/process.go`** — Change `executeClaudeInvocation` signature to return `(*runtypes.InvocationResult, error)`. The body simplifies to forwarding `invResult` directly instead of unpacking/repacking.

**`internal/runner/callbacks.go`** — Update the call site to access fields via `invResult.Result`, `invResult.Stats`, etc. Set `invResult.TimeoutType` directly on the shared struct instead of constructing a new one.

**Test files** — Update 4 test files that destructure the 5-value return to use struct field access instead:
- `process_test.go`
- `execute_claude_invocation_provider_result_test.go`
- `rate_limit_recovery_logging_test.go`
- `cross_review_routing_test.go`

### Scope

Pure mechanical refactoring. No behavioral changes, no new packages, no new files.

### Validation

`go build ./...` and `go test ./internal/runner/...` must pass.
