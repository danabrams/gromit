---
id: provider-type-rename
source_ideas: []
created: 2026-02-22
epic: provider-ecosystem
---

# Provider Type Rename: Remove Claude-Specific Naming from Generic Interfaces

## Specification

Several interfaces, types, and result structs in the codebase use "Claude" in their names despite being provider-agnostic abstractions. This creates confusion about whether the code is Claude-specific or generic, and makes multi-provider work feel like an afterthought. This spec renames these types to provider-neutral names.

### Why

When `CodexProvider` returns a `claude.Result`, or an escalation handler accepts `*claude.Result` from a Codex invocation, the naming is actively misleading. Developers (including LLM agents) waste time investigating whether these are genuinely Claude-specific or just poorly named. Clean naming makes the provider abstraction legible.

### Renames

#### 1. `pipeline.ClaudeClient` → `pipeline.LLMClient`

**File:** `internal/pipeline/pipeline.go`

```go
// Before
type ClaudeClient interface {
    Run(prompt string, model string) (*ClaudeRunResult, error)
}

// After
type LLMClient interface {
    Run(prompt string, model string) (*LLMRunResult, error)
}
```

Update the `Deps` struct field: `ClaudeClient` → `LLMClient`.

Update all implementations and call sites.

#### 2. `pipeline.ClaudeRunResult` → `pipeline.LLMRunResult`

**File:** `internal/pipeline/pipeline.go`

Rename the struct and update all references. The struct fields stay the same.

#### 3. `claude.Result` usage in escalation → `provider.Result`

**Files:**
- `internal/runner/escalation/handler.go` — `HandleEscalation`, `AnalyzeAndHandleFailure` parameter types
- `internal/runner/execution/invoker.go` — the conversion from `provider.Result` to `claude.Result` (eliminate the conversion entirely)

Currently `invoker.go` wraps `provider.Result` into `claude.Result` at line ~185 just to satisfy the escalation handler's signature. After this rename, the escalation handler accepts `*provider.Result` directly, and the wrapping is deleted.

#### 4. `claude.Result` usage in validation → `provider.Result`

**File:** `internal/runner/validation/runner.go`

`RunDirect()` returns `*claude.Result` but doesn't invoke Claude — it runs shell commands. Change the return type to `*provider.Result`. Update callers.

#### 5. Adapter type renames in `cmd/gromit/`

Rename adapter types that use "claude" in their names when they're actually provider-generic:
- `claudeClientAdapter` → `llmClientAdapter` (where it wraps a router, not specifically Claude)

Adapters that genuinely wrap a `claude.Client` (for the no-providers fallback path) can keep their names since they're accurately described.

### What NOT to Rename

- **`internal/claude/` package** — This package IS Claude-specific. It wraps the Claude CLI. Keep the name.
- **`claude.Client`** — The concrete Claude CLI client. Keep.
- **`provider.ClaudeProvider`** — The Claude-specific provider implementation. Keep.
- **`provider.CodexProvider`** — The Codex-specific provider implementation. Keep.
- **`cfg.Claude` config section** — Claude-specific CLI config. Keep.
- **Agent names** (`"claude"`, `"codex"`) — These are literal provider identifiers. Keep.

The rule: rename things that pretend to be generic but say "Claude." Keep things that are genuinely Claude-specific.

## Acceptance Criteria

- `pipeline.ClaudeClient` renamed to `pipeline.LLMClient` with all call sites updated
- `pipeline.ClaudeRunResult` renamed to `pipeline.LLMRunResult` with all references updated
- Escalation handler accepts `*provider.Result` instead of `*claude.Result`
- Validation runner returns `*provider.Result` instead of `*claude.Result`
- The `provider.Result` → `claude.Result` conversion in `invoker.go` is eliminated
- All tests pass after the rename
- No functional behavior changes — this is a pure rename/type-change refactor

## Decisions

1. **`LLMClient` not `ProviderClient`.** The pipeline interface is about invoking an LLM, not about the provider abstraction. `LLMClient` is clearer about what it does. The `provider.Router` is the provider abstraction; the pipeline just needs "something that runs prompts."

2. **One PR, mechanical rename.** This is a refactor with no behavior change. Do it in one pass with find-and-replace + compile verification. Splitting it across multiple PRs would create interim states where some code says Claude and some says LLM, which is worse than either extreme.

3. **Escalation uses `provider.Result` directly.** Rather than creating yet another result type, reuse the existing `provider.Result` which already has all the right fields. The `claude.Result` → `provider.Result` conversion that currently exists in `invoker.go` proves they have the same shape.

## Research & Context

### Affected Files (estimated)

- `internal/pipeline/pipeline.go` — interface + result type definition
- `internal/pipeline/decompose.go` — uses `ClaudeClient`
- `internal/pipeline/refine.go` — uses `ClaudeClient`
- `internal/pipeline/plan.go` — uses `ClaudeClient`
- `internal/pipeline/review.go` — uses `ClaudeClient`
- `internal/pipeline/verify_spec.go` — uses `ClaudeClient`
- `internal/runner/escalation/handler.go` — `*claude.Result` params
- `internal/runner/execution/invoker.go` — `claude.Result` wrapping
- `internal/runner/validation/runner.go` — `*claude.Result` return
- `cmd/gromit/decompose.go` — adapter type
- `cmd/gromit/review.go` — adapter type
- `cmd/gromit/main.go` — adapter type
- Various `*_test.go` files for all of the above

### Related Specs

- `provider-parity-decompose-retro` — wires decompose/retro into the router. Should land first so the adapter types exist before renaming them.
- `multi-provider-routing` — established the provider/router abstraction. This spec cleans up naming debt left behind.
- `invocation-result-consolidation` — may overlap if it also addresses `claude.Result` usage; check for conflicts.
