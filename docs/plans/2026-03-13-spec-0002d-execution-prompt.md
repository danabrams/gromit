# Spec 0002d — Execution Prompt (Multi-Provider Routing)

> **REQUIRED:** Use `superpowers:subagent-driven-development` to implement this plan, parallelizing independent tasks via subagents.
> **Plan document:** `docs/plans/2026-03-13-spec-0002d-implementation-plan.md`
> **Design document:** `docs/plans/2026-03-13-spec-0002c-0002d-design.md`
> **Test plans:** `docs/plans/2026-03-13-spec-0002c-0002d-testing-plan.md` and `docs/plans/2026-03-13-spec-0002c-0002d-manual-test-plan.md`

---

## Execution Workflow — MUST FOLLOW

**You implement ONE PHASE per conversation.** After each phase:

### Phase → Review Loop → CONTINUE.md → Context Clear

```
1. Implement the phase (TDD: red tests → green implementation → commit)
2. Run phase checkpoint tests
3. REVIEW ROUND 1: Use superpowers:code-reviewer to do a thorough review of all code written in this phase
   - Fix every Critical and Important issue found
   - Commit fixes
4. If issues were found in round 1:
   REVIEW ROUND 2: Run superpowers:code-reviewer again on the same scope
   - Fix every Critical and Important issue found
   - Commit fixes
5. If issues were found in round 2:
   REVIEW ROUND 3: Run superpowers:code-reviewer again (FINAL round)
   - Fix every Critical and Important issue found
   - Commit fixes
6. Run phase checkpoint tests one final time to confirm everything passes
7. Write progress to CONTINUE.md (see format below)
8. STOP — tell user the phase is complete and context can be cleared
```

**Review scope:** All files created or modified during the current phase. Compare against the phase's task descriptions in the implementation plan.

**Early exit:** If a review round finds ZERO issues, skip remaining rounds and proceed to CONTINUE.md.

### CONTINUE.md Format

Write `CONTINUE.md` to the worktree root with this structure:

```markdown
# Spec 0002d — Continue

## Status
- **Current phase:** Phase N — COMPLETE
- **Next phase:** Phase N+1 — [phase title]
- **Date:** [today's date]

## Completed Phases
- Phase 1: [title] — COMPLETE ([N] tests passing)
- Phase 2: [title] — COMPLETE ([N] tests passing)
- ...

## Phase [N] Summary
- Files created: [list]
- Files modified: [list]
- Tests added: [count]
- Review rounds: [1-3], issues fixed: [count]
- Final checkpoint: PASS

## Next Phase Instructions
1. Read this file
2. Read the execution prompt: docs/plans/2026-03-13-spec-0002d-execution-prompt.md
3. Read the implementation plan: docs/plans/2026-03-13-spec-0002d-implementation-plan.md
4. Skip to "Phase [N+1]" section below
5. Implement Phase [N+1] following the Phase → Review Loop → CONTINUE.md workflow

## Verification
[paste the actual test output from the final checkpoint run]
```

---

## Project Context

- **Language/runtime:** Go 1.26, CLI built with `github.com/spf13/cobra`
- **Project root:** `/Users/dabrams/gromit`
- **Module path:** `github.com/danabrams/gromit`
- **CLI entry point:** `cmd/gromit-next/main.go`
- **Provider abstraction:** `internal/provider/` -- `Provider` interface, `Router`, `CodexProvider`, tier constants
- **0002c prerequisite:** Spec 0002c (provider-agnostic adapter layer) **must be complete** before 0002d work begins. See "0002c Prerequisites" section below.

### Conventions

- **Nil-field normalization:** Use exported `NormalizeNilFields()` for cross-package types; use unexported `normalizeNilFields()` for internal-only types. Both map nil slices/maps to empty values.
- **Tests:** Co-located (`foo_test.go` next to `foo.go`), table-driven, fakes over mocks.
- **TDD:** Write a failing test FIRST, then implement to make it pass, then commit.

---

## Architecture Overview

0002d replaces the hardcoded Claude provider in `RealStageProvider` with the existing `provider.Router`, enabling Codex (and future providers) as alternatives with automatic fallback on usage-limit errors.

### Three-Layer Design (with Routing)

```
Pipeline Stages (specloop/stages/)
        |
        v
Per-Domain Adapters (planner/, review/, acceptor/, specloop/)
        |
        v
FallbackAdapter (llmadapter/)  <-- NEW in 0002d
        |
        v
Router (provider/)  <-- WIRED in 0002d
        |
        v
provider.Provider (claude, codex)
```

**Key behavior:**
- `FallbackAdapter` wraps `RouterSelector` and satisfies `ProviderAwareInvoker`
- Provider selection is **lazy** -- deferred to first `Invoke` call, not pipeline build time
- On usage-limit error: marks primary unavailable via `Router.MarkUnavailable`, then calls `Router.Select` again for fallback
- **Single-hop fallback only** -- primary to one fallback. N-hop is out of scope.
- Domain adapters are completely unaware of fallback -- same `Invoker` interface throughout

---

## Key Design Decisions

1. **`RouterSelector` interface** -- defined in `llmadapter/fallback.go`, abstracts `Router.Select` and `Router.MarkUnavailable`. The real `provider.Router` satisfies it. Tests use mock implementations.

2. **Lazy init with `sync.Mutex`** -- `FallbackAdapter` uses mutex-guarded lazy initialization (not `sync.Once`) so the primary provider can be re-resolved if previously unavailable (recovery-after-cooldown semantics).

3. **Usage-limit detection** -- delegates to `provider.IsUsageLimitError(result, err)`. Non-usage-limit errors propagate without fallback.

4. **`RoutingConfig` in `Policy`** -- new struct added to `execpolicy.Policy`:
   ```go
   type RoutingConfig struct {
       Preferences     map[string]string  // phase -> provider name or "any"
       Ratio           map[string]int     // provider name -> percentage (must sum to 100)
       CooldownSeconds int                // seconds before retrying unavailable provider
   }
   ```

5. **Router.Select double-counting** -- known limitation. `Select` increments invocation count as a side effect. On the fallback path, the failed primary's count is already incremented. Documented, not fixed in 0002d.

6. **Streaming fallback out of scope** -- `FallbackAdapter` only implements `Invoke`-based fallback. `InvokeStream` fallback deferred to future spec.

---

## 0002c Prerequisites

The following must exist from Spec 0002c before any 0002d work begins. **Verify these exist before starting:**

| Artifact | Package | What it provides |
|----------|---------|-----------------|
| `ProviderAwareInvoker` interface | `internal/next/llmadapter/` | Extends `Invoker` with `Provider() provider.Provider` |
| `llmadapter.New()` constructor | `internal/next/llmadapter/` | Creates `*LLMAdapter` from `provider.Provider` + `Config` |
| `*LLMAdapter` satisfies `ProviderAwareInvoker` | `internal/next/llmadapter/` | Has `Provider() provider.Provider` method |
| `llmadapter.Config` struct | `internal/next/llmadapter/` | Tier, Timeout, OnCost fields |
| `llmadapter.Invoker` interface | `internal/next/llmadapter/` | `Invoke(ctx, prompt) (*provider.Result, error)` |
| `llmadapter.ExtractJSON()` utility | `internal/next/llmadapter/` | JSON extraction from LLM output |
| Per-domain provider agents | `planner/`, `review/`, `acceptor/`, `specloop/` | `ProviderPlanAgent`, `ProviderReviewAgent`, `ProviderAcceptAgent`, `ProviderTaskRunner` |
| Contract test suites | Same packages | `RunPlanAgentContract`, `RunReviewAgentContract`, `RunAcceptAgentContract`, `RunTaskRunnerContract` |
| `RealStageProvider` wired with Claude | `cmd/gromit-next/stage_provider.go` | Hardcoded Claude provider via `llmadapter.New()` |

**Verification command:**

```bash
cd /Users/dabrams/gromit && go build ./internal/next/llmadapter/ && go test ./internal/next/llmadapter/ -run TestLLMAdapter -v -count=1
```

If this fails, 0002c is not complete -- stop and complete 0002c first.

---

## Implementation Phases with Subagent Parallelization

### Phase 1: FallbackAdapter (Tasks 1-2) -- Single Agent

> Sequential TDD: write failing tests, then implement. Single agent because Task 2 depends on Task 1.

**Task 1: FallbackAdapter failing tests**

- Create `internal/next/llmadapter/fallback_test.go`
- Tests: `TestFallbackAdapter_NormalInvocation_NoFallback`, `TestFallbackAdapter_UsageLimit_FallsBackToRouter`, `TestFallbackAdapter_NonUsageLimitError_NoFallback`, `TestFallbackAdapter_AllProvidersExhausted_ReturnsError`, `TestFallbackAdapter_SatisfiesProviderAwareInvoker`, `TestFallbackAdapter_Provider_ReturnsPrimaryProvider`
- Mock types: `mockProvider`, `mockProviderWithUsageLimit`, `mockRouter`, `mockSelectResult`
- See implementation plan Task 1 for exact test code
- Run: `go test ./internal/next/llmadapter/ -run TestFallbackAdapter -v -count=1` -- expect FAIL
- Commit: `red: FallbackAdapter tests for usage-limit failover`

**Task 2: FallbackAdapter implementation**

- Create `internal/next/llmadapter/fallback.go`
- Define `RouterSelector` interface (abstracts `Router.Select` + `Router.MarkUnavailable`)
- Implement `FallbackAdapter` struct with lazy init, single-hop fallback, usage-limit detection
- Compile-time checks: `var _ Invoker = (*FallbackAdapter)(nil)` and `var _ ProviderAwareInvoker = (*FallbackAdapter)(nil)`
- **Do NOT redefine `ProviderAwareInvoker`** -- it is defined by 0002c
- Run: `go test ./internal/next/llmadapter/ -v -count=1` -- expect PASS
- Commit: `green: FallbackAdapter with transparent usage-limit failover`

**Checkpoint:**

```bash
cd /Users/dabrams/gromit && go test ./internal/next/llmadapter/ -v -count=1
go vet ./internal/next/llmadapter/
```

**→ Now run the Review Loop (up to 3 rounds), then write CONTINUE.md and STOP.**

---

### Phase 2: Two Subagents in Parallel (Tasks 3 and 4)

> Use `superpowers:subagent-driven-development` to run these two tasks concurrently.

#### Subagent A: RealStageProvider Router Wiring + RoutingConfig (Task 3)

**Scope:** Wire the Router into `RealStageProvider.BuildStages` and add `RoutingConfig` to Policy.

**Files to modify:**
- `internal/next/execpolicy/policy.go` -- add `RoutingConfig` struct, add `Routing` field to `Policy`, add defaults in `DefaultPolicy()`, add validation (ratio sums to 100), add `NormalizeNilFields`
- `internal/next/execpolicy/policy_test.go` -- add `TestPolicy_Validate_RoutingRatioSumsTo100`, `TestPolicy_Validate_RoutingRatioValid`
- `cmd/gromit-next/stage_provider.go` -- add provider fields (`claudeProvider`, `codexProvider`, `stateFn`, `circuitBreaker`), add `buildRouter()` method, replace hardcoded Claude with `FallbackAdapter` wrapping Router in `BuildStages`
- `cmd/gromit-next/stage_provider_test.go` -- add `TestBuildRouter_ReturnsConfiguredRouter`, `TestBuildStages_NilCodexProvider_SingleProviderMode`
- `cmd/gromit-next/exec.go` -- construct providers and pass to `RealStageProviderConfig`

**Key details:**
- `BuildStages` creates one Router via `buildRouter(policy)`, then creates `FallbackAdapter` per phase (plan, execute, review, accept)
- Provider selection is lazy -- `BuildStages` does NOT call `router.Select`
- `RoutingConfig` defaults: all phases "any", ratio `{"claude": 100}`, cooldown 300s
- Validation: ratio values must sum to 100; unknown provider name validation deferred to router construction

**Run:** `go test ./cmd/gromit-next/ -v -count=1 && go test ./internal/next/execpolicy/ -v -count=1`

**Commit:** `feat: wire Router into RealStageProvider for multi-provider routing`

#### Subagent B: Codex Contract Tests (Task 4)

**Scope:** Add Codex contract tests alongside existing Claude contract tests.

**Cross-plan dependency:** Requires 0002c contract test suites (`RunPlanAgentContract`, `RunReviewAgentContract`, `RunAcceptAgentContract`, `RunTaskRunnerContract`) to exist. If they do not exist, this subagent must wait or skip.

**Files to modify:**
- `internal/next/planner/agent_contract_test.go` -- add `TestContract_ProviderPlanAgent_Codex`
- `internal/next/review/agent_contract_test.go` -- add `TestContract_ProviderReviewAgent_Codex`
- `internal/next/acceptor/agent_contract_test.go` -- add `TestContract_ProviderAcceptAgent_Codex`
- `internal/next/specloop/taskrunner_contract_test.go` -- add `TestContract_ProviderTaskRunner_Codex`

**Key details:**
- Each test gated by `GROMIT_LLM_CONTRACT=1` env var
- Wire: codex binary -> `provider.NewCodexProvider(...)` -> `llmadapter.New()` -> domain adapter -> contract suite
- If contract fails, investigate prompt compatibility -- Codex may need explicit JSON output instructions

**Run:** `GROMIT_LLM_CONTRACT=1 go test -tags llmcontract ./internal/next/planner/ -run TestContract_ProviderPlanAgent_Codex -v -count=1 -timeout 120s`

**Commit:** `feat: Codex contract tests -- validate provider compatibility`

**Phase 2 Checkpoint:**

```bash
cd /Users/dabrams/gromit
go test ./internal/next/execpolicy/ -v -count=1
go test ./cmd/gromit-next/ -v -count=1
go vet ./internal/next/... ./cmd/gromit-next/...
```

**→ Now run the Review Loop (up to 3 rounds), then write CONTINUE.md and STOP.**

---

### Phase 3: Integration Test Scaffolds (Task 5) -- Single Agent

> Depends on Phase 2 (needs Router wiring and contract test framework).

**Files to modify:**
- `cmd/gromit-next/stage_provider_test.go` -- add `TestIntegration_BuildStages_FallbackAdapter_RouterWiring` (must be in package main to access `RealStageProvider`)
- `internal/next/specloop/integration_test.go` -- add `TestIntegration_FallbackAdapter_UsageLimitFallback_ThroughRouter` (uses real Router, not mock)

**Key details:**
- BuildStages wiring test: construct `RealStageProvider` with mock providers, verify stages are created with correct adapters
- FallbackAdapter-through-Router test: primary hits usage limit, Router routes to fallback codex provider, verify fallback output
- Add skipped scaffolds for `TestIntegration_ProviderFallbackOnUsageLimit` and `TestIntegration_RouterPhasePreferences` (gated by `GROMIT_LLM_CONTRACT=1`)

**Run:** `go test ./cmd/gromit-next/ -v -count=1 && go test ./internal/next/specloop/ -v -count=1`

**Commit:** `feat: integration test scaffolds for multi-provider routing scenarios`

**Phase 3 Checkpoint:**

```bash
cd /Users/dabrams/gromit
go test ./internal/next/... -v -count=1
go test ./cmd/gromit-next/ -v -count=1
```

**→ Now run the Review Loop (up to 3 rounds), then write CONTINUE.md and STOP.**

---

### Phase 4: Final Verification (Task 6) -- Single Agent

No new code. Run full verification suite.

```bash
cd /Users/dabrams/gromit
go test ./internal/next/... ./cmd/gromit-next/ -v -count=1
go test -race ./internal/next/llmadapter/ -count=1
go vet ./internal/next/... ./cmd/gromit-next/...
gofmt -l internal/next/ cmd/gromit-next/
go build ./cmd/gromit-next/
```

For contract tests (local only, costs money):

```bash
GROMIT_LLM_CONTRACT=1 go test -tags llmcontract ./internal/next/... -run TestContract -v -count=1 -timeout 300s
```

**→ Now run the Review Loop (up to 3 rounds), then write FINAL CONTINUE.md and STOP.**

---

## Checkpoints

| Phase | Verification | Expected |
|-------|-------------|----------|
| Phase 1 | `go test ./internal/next/llmadapter/ -v -count=1` | PASS (FallbackAdapter tests green) |
| Phase 2A | `go test ./cmd/gromit-next/ ./internal/next/execpolicy/ -v -count=1` | PASS (Router wiring + RoutingConfig) |
| Phase 2B | `GROMIT_LLM_CONTRACT=1 go test -tags llmcontract ./internal/next/... -run TestContract.*Codex -v -count=1 -timeout 300s` | PASS (Codex contracts) |
| Phase 3 | `go test ./internal/next/... ./cmd/gromit-next/ -v -count=1` | PASS (all unit + integration) |
| Phase 4 | Full verification suite (see above) | All PASS, no race conditions, clean vet/fmt |

---

## Execution Rules

1. **TDD strictly.** Write a failing test first, then implement to pass it, then commit. No implementation without a covering test.
2. **Commit frequently.** After each green test or small logical unit.
3. **Do NOT redefine `ProviderAwareInvoker`.** It is defined by 0002c in `internal/next/llmadapter/`. Import and use it.
4. **Do NOT redefine `Invoker`.** Same -- defined by 0002c.
5. **Follow existing patterns** from `internal/provider/router.go` for Router usage and from `internal/next/llmadapter/` for adapter patterns.
6. **Run tests:** `go test ./internal/next/... -v` and `go test ./cmd/gromit-next/ -v`
7. **Build:** `go build ./cmd/gromit-next/`
8. **Interfaces for dependencies.** `RouterSelector` interface in `llmadapter/` abstracts the Router for testability.
9. **NormalizeNilFields convention.** Add `NormalizeNilFields()` to `RoutingConfig` (exported -- cross-package type). Call it from `Policy.NormalizeNilFields()`.

---

## What NOT to Do

- **Don't modify 0002c interfaces.** `Invoker`, `ProviderAwareInvoker`, `LLMAdapter`, and per-domain adapters are owned by 0002c. Do not change their signatures.
- **Don't add N-hop fallback.** Single-hop only (primary -> one fallback). If both fail, return error.
- **Don't add streaming fallback.** `FallbackAdapter.Invoke` only. `InvokeStream` fallback is a future spec.
- **Don't modify `internal/provider/router.go`.** The Router already has `Select`, `MarkUnavailable`, `RecordInvocation`, `RecordOutcome`. Use them as-is.
- **Don't modify `internal/provider/codex.go`.** The `CodexProvider` already exists from 0001.
- **Don't add provider-specific logic to domain adapters.** Domain adapters talk to `Invoker` -- they do not know which provider is underneath.
- **Don't add parallel provider invocation.** One provider at a time. Fallback is sequential.
- **Don't modify existing `internal/v2/` packages.**
- **Don't over-engineer.** Simple implementations that pass tests are better than clever abstractions.

---

## Success Criteria

All of these must be satisfied before the implementation is complete:

1. **FallbackAdapter works** -- usage-limit error on primary triggers transparent fallback to next available provider; non-usage-limit errors propagate normally; all-exhausted returns clear error
2. **Router wired into RealStageProvider** -- `BuildStages` creates `FallbackAdapter` per phase wrapping a single shared Router instance; provider selection is lazy (deferred to first `Invoke`)
3. **RoutingConfig in Policy** -- `Preferences`, `Ratio`, `CooldownSeconds` fields with defaults; ratio validation (sums to 100); `NormalizeNilFields` wired
4. **Codex contract tests pass** -- all four domain contract suites (`Plan`, `Review`, `Accept`, `TaskRunner`) pass against Codex provider locally
5. **Single-provider mode preserved** -- when `codexProvider` is nil, Router operates with Claude only; no behavioral change from pre-0002d
6. **Integration tests scaffold** -- BuildStages wiring test + FallbackAdapter-through-Router test both pass with mock providers
7. **All existing tests still pass** -- zero regressions in `./internal/next/...` and `./cmd/gromit-next/`

---

## Final Verification

After all phases are complete, run the full verification:

```bash
cd /Users/dabrams/gromit
go test ./internal/next/... -v -count=1
go test ./cmd/gromit-next/ -v -count=1
go test -race ./internal/next/llmadapter/ -count=1
go vet ./internal/next/... ./cmd/gromit-next/...
go mod tidy && git diff --exit-code go.mod go.sum
go build ./cmd/gromit-next/
gofmt -l internal/next/ cmd/gromit-next/
```

Only push after all checks pass.
