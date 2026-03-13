# Spec 0002c — Execution Prompt

> **REQUIRED:** Use `superpowers:subagent-driven-development` to execute independent tasks in parallel.
> **Plan document:** `docs/plans/2026-03-13-spec-0002c-implementation-plan.md`
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
# Spec 0002c — Continue

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
2. Read the execution prompt: docs/plans/2026-03-13-spec-0002c-execution-prompt.md
3. Read the implementation plan: docs/plans/2026-03-13-spec-0002c-implementation-plan.md
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
- **Provider abstraction:** `internal/provider/` — supports tiers `low`, `medium`, `high`, `xhigh`
- **Existing stage interfaces (satisfy, do not modify):** `internal/next/specloop/stages/` — `SpecCompiler`, `PlanCreator`, `FinalValidator`, `ReviewRunner`, `AcceptEvaluator`
- **Existing domain interfaces (satisfy, do not modify):** `planner.Agent`, `review.ReviewAgent`, `acceptor.AcceptAgent`, `specloop.TaskRunner`
- **Wiring target:** `cmd/gromit-next/stage_provider.go` — `RealStageProvider.BuildStages` currently uses noop implementations

### Conventions

- **Nil-field normalization:** Use exported `NormalizeNilFields()` for cross-package types; use unexported `normalizeNilFields()` for internal-only types.
- **Tests:** Co-located (`foo_test.go` next to `foo.go`), table-driven, fakes over mocks.
- **TDD:** Write a failing test FIRST, then implement to make it pass, then commit.

---

## Architecture Overview

Three-layer adapter design replacing noops with real LLM-backed implementations:

```
Pipeline Stages (specloop/stages/)
        |
        v
Per-Domain Adapters (planner/, review/, acceptor/, specloop/, validator/)
        |
        v
Shared LLMAdapter (llmadapter/)
        |
        v
provider.Provider (provider/)
```

Each layer has a single responsibility:

1. **Pipeline stages** — orchestration, state transitions, replan logic (already built in 0002a/0002b, do not modify)
2. **Per-domain adapters** — parse provider output into domain types (findings, plans, criteria). These are the NEW code in 0002c.
3. **Shared LLMAdapter** — invoke provider, enforce timeout, track cost. NEW in 0002c.
4. **Provider** — CLI execution, streaming, error detection (already built in 0001, do not modify)

---

## Key Design Decisions

Read the full design doc before starting: `docs/plans/2026-03-13-spec-0002c-0002d-design.md`

### No retry in LLMAdapter

Retry strategies differ per domain (parse-error retry in planner, transient retry in acceptor, escalation in task runner). LLMAdapter only does: invoke provider, enforce timeout, track cost. Callers own retry logic.

### Invoker interface

All per-domain adapters depend on `llmadapter.Invoker` (not `*LLMAdapter` directly). This enables mock substitution in tests:

```go
type Invoker interface {
    Invoke(ctx context.Context, prompt string) (*provider.Result, error)
}
```

### ProviderAwareInvoker

Extends `Invoker` with `Provider() provider.Provider`. `LLMAdapter` satisfies it directly. Used by 0002d's `FallbackAdapter` — define the interface in 0002c but do not build `FallbackAdapter`.

### ExtractJSON utility

Shared `llmadapter.ExtractJSON(s string) string` — extracts first JSON object or array from LLM output. Handles markdown fences. Used by `ProviderReviewAgent` and `ProviderAcceptAgent` to parse LLM responses.

### Tier parameter semantics

Tier is determined at adapter construction time and baked into `LLMAdapter.Config.Tier`. The `tier` parameter in domain adapter `Invoke` signatures exists solely for interface compatibility — the adapter ignores it in favor of its configured tier. A mismatch is logged once at debug level via `sync.Once`.

### Result returned on error

`LLMAdapter.Invoke` returns the provider result even when an error occurs. This enables 0002d's `FallbackAdapter` to call `IsUsageLimitError(result, err)` for provider-aware routing.

---

## Existing Patterns to Study

Read these files before implementing to understand existing interfaces and patterns:

| File | What to learn |
|------|--------------|
| `internal/provider/provider.go` | `Provider` interface, tier constants, `Result` type |
| `internal/next/planner/planner.go` | `Agent` interface, `AgentResult` type, `PlanRequest` |
| `internal/next/review/runner.go` | `ReviewAgent` interface, `Finding` type, `ParseError` |
| `internal/next/acceptor/evaluator.go` | `AcceptAgent` interface, `CriterionResult` type |
| `internal/next/specloop/taskloop.go` | `TaskRunner` interface, `TaskResult` type |
| `internal/next/specloop/stages/validate.go` | `FinalValidator` interface |
| `internal/next/validator/runner.go` | `Runner` type, `Check`, `FinalResult` |
| `internal/next/contextpkt/compiler.go` | `DefaultCompiler`, `Compile()` signature, `Packet` type |
| `cmd/gromit-next/stage_provider.go` | Current noop wiring in `RealStageProvider.BuildStages` |

---

## Implementation Phases with Subagent Parallelization

### Phase 1: LLMAdapter package (sequential, single agent)

**Tasks 1-2** from the plan. Foundation that all other work depends on.

1. Create `internal/next/llmadapter/adapter_test.go` with failing tests (Task 1)
2. Create `internal/next/llmadapter/adapter.go` with implementation (Task 2)
3. Run tests: `go test ./internal/next/llmadapter/ -v -count=1`
4. Commit after red, commit after green.

**Key behaviors to implement:**
- `Invoke` delegates to `provider.Run` with configured tier
- `InvokeStream` delegates to `provider.StreamRun` with configured tier
- Timeout enforcement via `context.WithTimeout`
- `OnCost` callback fires on non-zero cost
- `ProviderName()` and `Tier()` accessors
- Result returned even on error (for 0002d compatibility)

**Checkpoint:**
```bash
go test ./internal/next/llmadapter/ -v -count=1
```

**→ Now run the Review Loop (up to 3 rounds), then write CONTINUE.md and STOP.**

---

### Phase 2: Invoker interfaces + ExtractJSON (sequential, single agent)

**Tasks 4a, 4b** from the plan. Depends on Phase 1.

1. Create `internal/next/llmadapter/invoker.go` — define `Invoker` and `ProviderAwareInvoker` interfaces, `ProviderAware` wrapper (Task 4a)
2. Add `Provider()` method to `LLMAdapter` so it satisfies `ProviderAwareInvoker` directly
3. Add compile-time checks: `var _ Invoker = (*LLMAdapter)(nil)` and `var _ ProviderAwareInvoker = (*LLMAdapter)(nil)`
4. Create `internal/next/llmadapter/parse.go` + `parse_test.go` — `ExtractJSON` utility (Task 4b)
5. Run tests: `go test ./internal/next/llmadapter/ -v -count=1`

**Checkpoint:**
```bash
go test ./internal/next/llmadapter/ -v -count=1
```

**→ Now run the Review Loop (up to 3 rounds), then write CONTINUE.md and STOP.**

---

### Phase 3: Per-domain adapters IN PARALLEL via subagents

Use `superpowers:subagent-driven-development` to run six subagents in parallel. Each subagent implements one adapter pair (red test + green implementation). All depend on Phase 2 (the `llmadapter` package must exist).

**IMPORTANT:** Each subagent works in a different package. There are no file-level conflicts between subagents.

#### Subagent A: ProviderPlanAgent (Tasks 3, 4b planner part)

**Package:** `internal/next/planner/`

**Files to create:**
- `internal/next/planner/provider_agent_test.go` (red)
- `internal/next/planner/provider_agent.go` (green)

**Interface to satisfy:** `planner.Agent` (defined in `planner/planner.go`)
```go
type Agent interface {
    Invoke(ctx context.Context, prompt string, tier string) (AgentResult, error)
}
```

**Behavior:**
- Accept `tier` param for interface compatibility but ignore it (use adapter's configured tier)
- Log tier mismatch once via `sync.Once`
- Map `provider.Result` fields to `AgentResult{Output, TokensIn, TokensOut, Cost, Model, Duration}`
- Add compile-time check: `var _ Agent = (*ProviderPlanAgent)(nil)`

**Subagent instructions:**
```
Implement ProviderPlanAgent in internal/next/planner/.
Study internal/next/planner/planner.go for the Agent interface and AgentResult type.
TDD: write failing test first (provider_agent_test.go), then implement (provider_agent.go).
The adapter depends on llmadapter.Invoker (in internal/next/llmadapter/invoker.go).
Use a mockLLMInvoker in tests that satisfies llmadapter.Invoker.
Commit after red tests, commit after green implementation.
Run: go test ./internal/next/planner/ -run TestProviderPlanAgent -v -count=1
```

#### Subagent B: ProviderReviewAgent (Tasks 5, 6)

**Package:** `internal/next/review/`

**Files to create:**
- `internal/next/review/provider_agent_test.go` (red)
- `internal/next/review/provider_agent.go` (green)

**Interface to satisfy:** `review.ReviewAgent` (defined in `review/runner.go`)
```go
type ReviewAgent interface {
    ReviewFacet(ctx context.Context, facetName string, prompt string) ([]Finding, error)
}
```

**Behavior:**
- Invoke LLM, extract JSON via `llmadapter.ExtractJSON`, unmarshal into `[]Finding`
- Return `ParseError` (not plain error) on malformed JSON or missing required fields — this is important because `Runner` retries on `ParseError`
- Handle markdown-fenced JSON output
- Add compile-time check: `var _ ReviewAgent = (*ProviderReviewAgent)(nil)`

**Subagent instructions:**
```
Implement ProviderReviewAgent in internal/next/review/.
Study internal/next/review/runner.go for ReviewAgent interface, Finding type, and ParseError.
Study internal/next/review/finding.go for Finding's custom UnmarshalJSON that validates required fields.
TDD: write failing test first, then implement.
Use llmadapter.ExtractJSON for JSON extraction (in internal/next/llmadapter/parse.go).
Return *ParseError on parse failures (not fmt.Errorf) — Runner retries on ParseError.
Use a mockInvoker in tests that satisfies llmadapter.Invoker.
Commit after red tests, commit after green implementation.
Run: go test ./internal/next/review/ -run TestProviderReviewAgent -v -count=1
```

#### Subagent C: ProviderAcceptAgent (Tasks 7, 8)

**Package:** `internal/next/acceptor/`

**Files to create:**
- `internal/next/acceptor/provider_agent_test.go` (red)
- `internal/next/acceptor/provider_agent.go` (green)

**Interface to satisfy:** `acceptor.AcceptAgent` (defined in `acceptor/evaluator.go`)
```go
type AcceptAgent interface {
    EvaluateCriterion(ctx context.Context, prompt string) (CriterionResult, error)
}
```

**Behavior:**
- Invoke LLM, parse output via `ParseCriterionResult` helper (uses `ExtractJSON` + `json.Unmarshal`)
- Call `NormalizeNilFields()` on parsed result
- Handle markdown-fenced JSON
- Add compile-time check: `var _ AcceptAgent = (*ProviderAcceptAgent)(nil)`

**Subagent instructions:**
```
Implement ProviderAcceptAgent in internal/next/acceptor/.
Study internal/next/acceptor/evaluator.go for AcceptAgent interface and CriterionResult type.
TDD: write failing test first, then implement.
Also implement ParseCriterionResult(output string) (CriterionResult, error) as a public helper.
Use llmadapter.ExtractJSON for JSON extraction.
Use a mockInvoker in tests that satisfies llmadapter.Invoker.
Commit after red tests, commit after green implementation.
Run: go test ./internal/next/acceptor/ -run TestProviderAcceptAgent -v -count=1
```

#### Subagent D: ProviderTaskRunner (Tasks 9, 10)

**Package:** `internal/next/specloop/`

**Files to create:**
- `internal/next/specloop/provider_taskrunner_test.go` (red)
- `internal/next/specloop/provider_taskrunner.go` (green)

**Interface to satisfy:** `specloop.TaskRunner` (defined in `specloop/taskloop.go`)
```go
type TaskRunner interface {
    RunTask(ctx context.Context, task runstore.Task) (TaskResult, error)
    RepairTask(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error)
}
```

**Behavior:**
- `RunTask`: render task prompt from `runstore.Task` fields, invoke LLM, map result
- `RepairTask`: render repair prompt including failure context, invoke LLM, map result
- Prompt must include task objective, expected touched area, proof checks
- Repair prompt must include failure details
- Map `provider.Result.Success` to status: `true` -> `"done"`, `false` -> `"failed"`
- `TokensUsed = InputTokens + OutputTokens`
- Add compile-time check: `var _ TaskRunner = (*ProviderTaskRunner)(nil)`

**Subagent instructions:**
```
Implement ProviderTaskRunner in internal/next/specloop/.
Study internal/next/specloop/taskloop.go for TaskRunner interface and TaskResult type.
Study internal/next/runstore/types.go for Task struct fields.
TDD: write failing test first, then implement.
Include capturingInvoker in tests to verify prompt content.
Commit after red tests, commit after green implementation.
Run: go test ./internal/next/specloop/ -run TestProviderTaskRunner -v -count=1
```

#### Subagent E: ShellValidator (Tasks 11, 12)

**Package:** `internal/next/validator/`

**Files to create:**
- `internal/next/validator/shell_validator_test.go` (red)
- `internal/next/validator/shell_validator.go` (green)

**Interface to satisfy:** `stages.FinalValidator` (defined in `specloop/stages/validate.go`)
```go
type FinalValidator interface {
    RunFinal(ctx context.Context, alwaysRun []validator.Check, projectChecks []validator.Check, workDir string) (validator.FinalResult, error)
}
```

**Behavior:**
- Pure delegation to `validator.Runner.RunFinal()` — no reimplementation of check logic
- No LLM dependency — this is shell-only validation
- Constructor takes `*Runner`, must not be nil

**Subagent instructions:**
```
Implement ShellValidator in internal/next/validator/.
Study internal/next/validator/runner.go for Runner type, Check type, FinalResult type.
Study internal/next/specloop/stages/validate.go for FinalValidator interface.
TDD: write failing test first, then implement.
ShellValidator is a thin wrapper: constructor takes *Runner, RunFinal delegates entirely.
Test with real Runner + shell commands (echo, exit 1).
Commit after red tests, commit after green implementation.
Run: go test ./internal/next/validator/ -run TestShellValidator -v -count=1
```

#### Subagent F: SpecCompilerAdapter (Tasks 13, 14)

**Package:** `internal/next/contextpkt/`

**Files to create:**
- `internal/next/contextpkt/spec_compiler_test.go` (red)
- `internal/next/contextpkt/spec_compiler_adapter.go` (green)

**Interface to satisfy:** `stages.SpecCompiler` (defined in `specloop/stages/compile.go`)
```go
type SpecCompiler interface {
    Compile(ctx context.Context) (string, error)
}
```

**Behavior:**
- Wraps `contextpkt.DefaultCompiler` (different signature: `Compile(ctx, cell, level, opts) (Packet, error)`)
- Captures cell resolution, level, and token budget at construction time
- Renders `Packet.Sections` as readable text string
- No LLM dependency — purely deterministic

**Subagent instructions:**
```
Implement SpecCompilerAdapter in internal/next/contextpkt/.
Study internal/next/contextpkt/compiler.go for DefaultCompiler, Compile signature, Packet type, Cell type.
Study internal/next/specloop/stages/compile.go for SpecCompiler interface.
TDD: write failing test first, then implement.
Adapter captures cell/level/budget at construction, Compile(ctx) needs no extra args.
Use inMemoryStore (satisfies ArtifactStore) in tests.
Commit after red tests, commit after green implementation.
Run: go test ./internal/next/contextpkt/ -run TestSpecCompilerAdapter -v -count=1
```

**Phase 3 Checkpoint (after all subagents complete):**
```bash
go test ./internal/next/planner/ -v -count=1
go test ./internal/next/review/ -v -count=1
go test ./internal/next/acceptor/ -v -count=1
go test ./internal/next/specloop/ -v -count=1
go test ./internal/next/validator/ -v -count=1
go test ./internal/next/contextpkt/ -v -count=1
go vet ./internal/next/...
```

**→ Now run the Review Loop (up to 3 rounds), then write CONTINUE.md and STOP.**

---

### Phase 4: RealStageProvider wiring (sequential, single agent)

**Task 16** from the plan. Depends on all Phase 3 subagents completing.

1. Update `cmd/gromit-next/stage_provider.go` to replace noop implementations with real adapters
2. Create Claude provider (hardcoded for 0002c)
3. Wire per-domain adapters with appropriate tiers from `policy.Models`
4. Wire cost callbacks to `budget.AddCost` (except execute adapter — see note below)
5. Write a test verifying `BuildStages` returns stages backed by real adapters

**Cost tracking note:** The execute adapter's `OnCost` callback must be `nil`. `RunTaskLoop` already calls `Budget.AddCost(result.Cost)` after each task, so adding an `OnCost` callback on the execute adapter would double-count costs.

**Wiring summary:**
| Stage | Adapter chain |
|-------|--------------|
| Compile | `contextpkt.NewSpecCompilerAdapter(cfg)` |
| Plan | `planner.NewProviderPlanAgent(planAdapter, tier)` -> `planner.NewPlanner(agent, tier)` |
| Execute | `specloop.NewProviderTaskRunner(execAdapter)` |
| Validate | `validator.NewShellValidator(validator.NewRunner())` |
| Review | `review.NewProviderReviewAgent(reviewAdapter)` -> `review.NewRunner(agent, config)` |
| Accept | `acceptor.NewProviderAcceptAgent(acceptAdapter)` -> `acceptor.NewEvaluator(agent)` |

**Checkpoint:**
```bash
go test ./cmd/gromit-next/ -v -count=1
go build ./cmd/gromit-next/
```

**→ Now run the Review Loop (up to 3 rounds), then write CONTINUE.md and STOP.**

---

### Phase 5: Contract test framework (sequential, single agent)

**Tasks 17, 18, 19** from the plan. Depends on Phase 4.

1. Create contract test files gated by build tag `//go:build llmcontract` and env var `GROMIT_LLM_CONTRACT=1`
2. Write `RunPlanAgentContract`, `RunReviewAgentContract`, `RunAcceptAgentContract`, `RunTaskRunnerContract` functions
3. Wire `buildReal*` helpers to real Claude provider
4. Create integration scenario scaffolds (skip-only stubs for now)

**Contract test assertions (structural only, no content quality):**
- Planner: output parseable as Plan, tasks have task_id and objective, token counts > 0
- Review: output parseable as `[]Finding`, findings have severity/file/description, empty findings returns `[]` not nil
- Acceptor: output parseable as `CriterionResult`, status is pass/fail/unclear, rationale non-empty on fail
- TaskRunner: RunTask returns TaskResult with status, TokensUsed > 0

**Checkpoint:**
```bash
# Contract tests compile but skip without env var:
go test -tags llmcontract ./internal/next/... -run TestContract -v -count=1

# Contract tests pass against real Claude (local only):
GROMIT_LLM_CONTRACT=1 go test -tags llmcontract ./internal/next/... -run TestContract -v -count=1 -timeout 300s
```

**→ Now run the Review Loop (up to 3 rounds), then write CONTINUE.md and STOP.**

---

### Phase 6: Verification checkpoint

Run full verification to confirm all work is integrated:

```bash
go test ./internal/next/... -v -count=1
go test ./cmd/gromit-next/ -v -count=1
go test -race ./internal/next/...
go vet ./internal/next/... && go vet ./cmd/gromit-next/...
go build ./cmd/gromit-next/
gofmt -l internal/next/
go mod tidy && git diff --exit-code go.mod go.sum
```

All must pass with zero output from `gofmt -l` and no `go.mod`/`go.sum` drift.

**→ Now run the Review Loop (up to 3 rounds), then write FINAL CONTINUE.md and STOP.**

---

## Checkpoints

| After Phase | Verification command |
|-------------|---------------------|
| 1 | `go test ./internal/next/llmadapter/ -v -count=1` |
| 2 | `go test ./internal/next/llmadapter/ -v -count=1` |
| 3 | `go test ./internal/next/planner/ ./internal/next/review/ ./internal/next/acceptor/ ./internal/next/specloop/ ./internal/next/validator/ ./internal/next/contextpkt/ -v -count=1` |
| 4 | `go test ./cmd/gromit-next/ -v -count=1 && go build ./cmd/gromit-next/` |
| 5 | `go test -tags llmcontract ./internal/next/... -run TestContract -v -count=1` (all skip) |
| 6 | Full verification suite (see Phase 6 above) |

---

## Execution Rules

1. **TDD strictly.** Write a failing test first, then implement to pass it, then commit. No implementation without a covering test.
2. **Commit frequently.** After each red test and each green implementation.
3. **DRY, YAGNI.** No speculative abstractions. Build only what the plan specifies.
4. **Follow existing interfaces exactly.** Domain adapters must satisfy existing interfaces without modification.
5. **Run tests:** `go test ./internal/next/... -v` after each phase.
6. **Build:** `go build ./cmd/gromit-next/` after Phase 4.
7. **Fakes over mocks.** Use `mockInvoker` (satisfies `llmadapter.Invoker`) in domain adapter tests. Use `mockProvider` (satisfies `provider.Provider`) in llmadapter tests.
8. **No global state.** All configuration flows through constructors and config structs.
9. **NormalizeNilFields convention.** Call `NormalizeNilFields()` on parsed results that have slice/map fields (e.g., `CriterionResult`).

---

## What NOT to Do

- **Don't modify existing stage implementations** in `internal/next/specloop/stages/`. Adapters satisfy existing interfaces; stages are unchanged.
- **Don't modify existing domain interfaces** (`planner.Agent`, `review.ReviewAgent`, `acceptor.AcceptAgent`, `specloop.TaskRunner`). Implement them, don't change them.
- **Don't modify `internal/provider/`** — the provider abstraction is stable.
- **Don't add 0002d routing.** No `FallbackAdapter`, no `Router` wiring, no multi-provider logic. Hardcode Claude in `RealStageProvider` for now.
- **Don't add retry logic to LLMAdapter.** Retry belongs in callers (planner, review runner, etc.).
- **Don't add streaming fallback.** `InvokeStream` fallback is out of scope.
- **Don't create new stage types.** Use existing stages with new adapter implementations.
- **Don't modify `internal/next/contextpkt/compiler.go`** — wrap `DefaultCompiler` in an adapter instead.

---

## Success Criteria

All of these must be satisfied before the implementation is complete:

1. **All unit tests pass:** `go test ./internal/next/... ./cmd/gromit-next/ -v` exits 0
2. **Contract test stubs compile:** `go test -tags llmcontract ./internal/next/... -run TestContract -v -count=1` compiles and skips
3. **RealStageProvider wires real adapters:** `BuildStages` returns stages backed by `ProviderPlanAgent`, `ProviderReviewAgent`, `ProviderAcceptAgent`, `ProviderTaskRunner`, `ShellValidator`, and `SpecCompilerAdapter` — no noop implementations remain for these six stages
4. **LLMAdapter satisfies both interfaces:** `var _ Invoker = (*LLMAdapter)(nil)` and `var _ ProviderAwareInvoker = (*LLMAdapter)(nil)` compile
5. **ExtractJSON handles common LLM output formats:** bare JSON, markdown-fenced JSON, prose-prefixed JSON
6. **No regressions:** existing tests in `internal/next/...` and `cmd/gromit-next/` continue to pass
7. **Clean formatting:** `gofmt -l internal/next/` produces no output
8. **No module drift:** `go mod tidy && git diff --exit-code go.mod go.sum`

---

## Final Verification

After all phases are complete, run the full verification:

```bash
go test ./internal/next/... -v -count=1
go test ./cmd/gromit-next/ -v -count=1
go test -race ./internal/next/...
go vet ./internal/next/... && go vet ./cmd/gromit-next/...
go build ./cmd/gromit-next/
gofmt -l internal/next/
go mod tidy && git diff --exit-code go.mod go.sum
```

Only push after all checks pass.
