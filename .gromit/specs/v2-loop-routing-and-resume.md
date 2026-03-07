---
id: v2-loop-routing-and-resume
source_ideas: []
created: 2026-03-07
depends_on: []
---

# V2 Loop Routing and Resume

## Specification

The v2 spec loop currently hardcodes Claude as the sole provider, ignores tier configuration, replans from scratch on resume, treats gate blocks as fatal failures, and assembles prompts without budget shaping. This spec ports five capabilities natively into the v2 architecture:

1. **Multi-provider routing** with ratio-based balancing, phase preferences, and fallback
2. **Tier-aware model selection** across all LLM-invoking phases
3. **Resume with gap analysis and selective revalidation** instead of replanning
4. **Topological bead ordering with re-queue on block** instead of immediate failure
5. **Prompt budget shaping** with phase-filtered rules, scoped context, and per-bead budget adjustment

All five changes share a design principle: the **loop layer owns routing and orchestration**, stages remain stateless and unaware of provider selection.

### Multi-Provider Routing

A new `internal/v2/routing/` package with a `Router` struct that implements the same 3-layer selection as v1's `internal/provider/router.go`:

1. Phase preferences (`routing.phase_preferences`) -- pin phases to providers
2. Ratio balancing (`routing.ratio`) -- spread `any`-phase work across providers
3. Fallback with cooldown (`routing.fallback`) -- mark unavailable on usage-limit errors

The router exposes:

```go
Select(phase, tier string) (llmtypes.LLMProvider, model string, err error)
MarkUnavailable(name string)
RecordInvocation(name string)
```

The router is constructed in `cmd/gromit/run2.go` from `cfg.Providers` and `cfg.Routing`. Multiple `LLMProvider` implementations (Claude, Codex) are registered by provider name.

### Tier-Aware Model Selection

Tier resolution lives in `internal/v2/routing/tier.go`. Given a phase name:

1. Check `methodology.phase_models[phase]` for a tier override (e.g., build -> "low")
2. Fall back to priority/labels via `BuildTierForStrategy()`
3. The router maps the abstract tier to a provider-specific model name

The **bead loop** is the routing seam. Before calling `stage.Run()`, the loop:

1. Determines the phase name from the stage
2. Determines the starting tier from config
3. Calls `Router.Select(phase, tier)` to get provider + model
4. Populates `StageRequest.Model` and a new `StageRequest.Provider` field
5. Stages use `req.Provider.Invoke()` if set, falling back to their constructed provider

The **spec loop** does the same for spec-level stages (plan, decompose, accept).

**Escalation:** The bead loop's existing retry loop owns escalation. On failure, it increments `RetryContext.EscalationLevel`, maps that to the next tier in the escalation chain, and re-calls `Router.Select(phase, nextTier)`. No separate escalation handler.

### Resume with Gap Analysis and Selective Revalidation

On resume (existing plan + beads found), the spec loop adds two steps before starting the bead loop:

1. **Gap analysis** -- Diff worktree HEAD against the last successful bead's commit. Identify completed beads whose files have changed. Cheap: file-level diff + bead-to-file mapping from commit history.

2. **Selective revalidation** -- Run validation commands scoped to flagged beads. If validation fails, mark the bead incomplete and re-queue it.

These run inside the spec loop (not as stages) because they're orchestration -- they produce a modified bead list that feeds into the existing bead loop.

The remediation worktree fix: `remediationRunner.Run()` signature expands to `Run(ctx, specID, worktree string)`, and the Request template gets the worktree populated.

### Topological Ordering and Gate Re-queue

Before the bead loop iterates, beads are sorted topologically by their `DependsOn`/`BlockedBy` graph. Beads with no dependencies come first.

When gate returns `DecisionBlock` (dependency not yet met), the bead is re-queued to the end of the current pass rather than failed. After a full pass with zero progress (all remaining beads blocked), the loop stops with a descriptive error.

Gate stage itself is unchanged -- it already returns Skip/Block/Proceed.

### Prompt Budget Shaping

Port v1's prompt shaping pipeline into v2's `PromptAssembler`:

1. **Phase-filtered rules** -- `loadBaseInstructions()` takes a phase parameter, filters RULES.md to phase-relevant sections, and caps at the phase-specific limit (build: 12,800 chars, red: 8,500 chars, etc.)
2. **Scoped project context** -- `loadProjectContext()` scopes CLAUDE.md to packages touched by the current bead (v1's `applyScopedClaudeContext()` logic)
3. **Budget shaping** -- New `ShapeBudget()` step in the assembler that applies a total character cap with scope-adjusted scaling (50% for 1-2 file beads, 75% for 3-4, 100% for 5+)
4. **Per-phase caps** -- Each phase gets its own budget from config, with the option to override via `prompt.budget.max_chars`

This changes `PromptAssembler.Assemble()` from a flat concatenator to a budget-aware assembler that trims layers to fit.

**Expected savings:** 75-80% reduction for small beads (1-2 files), 60-65% for medium beads (3-4 files), 50-55% for large beads (5+ files). Most decomposed beads are small, so typical savings are 4-5x.

## Acceptance Criteria

### Multi-Provider Routing
- When multiple providers are configured, `routing.ratio` and `routing.phase_preferences` control provider selection
- Provider invocation counts are tracked for ratio balancing
- When a provider hits a usage limit, the router marks it unavailable and falls back to the next provider
- After the configured cooldown period, the router re-enables the unavailable provider

### Tier-Aware Model Selection
- All LLM-invoking phases (plan, decompose, build, validate, review, accept, gap-analysis, triage) use tier selection from `methodology.phase_models` config
- Build phase starts at the configured tier (e.g., "low" -> haiku) not the default P1 model
- Escalation increments tier (low -> medium -> high) on build/validate failure within the bead loop's existing retry mechanism

### Resume
- On resume with existing plan and beads, the loop does not replan or re-decompose
- On resume, gap analysis identifies completed beads with changed files
- Selective revalidation re-checks only flagged beads and re-queues failures
- Remediation stages receive the correct worktree path

### Topological Ordering
- Beads are executed in topological dependency order
- When a bead's dependency hasn't completed, the bead is re-queued (not failed)
- After a full pass with no progress, the loop terminates with a clear blocked-dependency error

### Prompt Budget Shaping
- Build prompts are budget-shaped: total prompt size respects `prompt.budget.max_chars` when configured
- RULES.md is filtered to phase-relevant sections with per-phase character caps
- CLAUDE.md is scoped to packages relevant to the current bead
- Scope-adjusted budgets scale prompt size proportional to bead complexity (file count)
- Prompt shaping produces a `ShapeReport` for observability (what was trimmed, by how much)

## Decisions

1. **Loop-level routing, not stage-level.** The bead loop and spec loop own provider/tier selection. Stages receive a populated `StageRequest` with provider and model already chosen. This keeps stages stateless and routing-unaware.

2. **New `StageRequest.Provider` field over changing stage constructors.** Stages use `req.Provider` if set, falling back to their constructed default. This avoids changing every stage constructor and `AdapterSet` while enabling per-invocation routing.

3. **Escalation in the bead loop's retry loop, not a separate handler.** The bead loop already has retry logic with `RetryContext.EscalationLevel`. Adding tier escalation there avoids a separate wrapper layer and keeps retry/escalation in one place.

4. **Gap analysis as spec-loop orchestration, not a stage.** It produces a modified bead list (re-opened beads), which is orchestration input. Making it a stage would require it to return beads as artifacts and add unnecessary ceremony.

5. **Topological sort before iteration, re-queue on block.** Prevents most gate blocks upfront. Re-queue handles edge cases (failed dependencies) gracefully without retrying gate.

6. **Remediation signature expansion over worktree injection.** Adding `worktree` to `Run()` is the minimal contract change. Alternatives (context values, struct fields) hide the dependency.

7. **Port v1 prompt shaping natively into v2.** V2's `PromptAssembler` becomes budget-aware by integrating v1's `ShapeContextForBudget()`, phase-filtered rules, and scoped context. This is a native port, not a bridge to v1 code.

8. **Scope-adjusted budgets over fixed caps.** Small beads (1-2 files) get 50% of the budget cap; medium beads (3-4 files) get 75%; large beads get 100%. This matches v1's proven approach and prevents small tasks from drowning in irrelevant context.

## Architecture Direction

All changes live in the v2 package tree. New package: `internal/v2/routing/` (router + tier resolution). Modified packages: `internal/v2/loop/` (bead_loop routing seam, spec_loop resume logic, topological sort), `internal/v2/stage/` (StageRequest gains Provider field), `internal/v2/prompt/` (budget-aware assembler), `internal/v2/remediation/` (worktree propagation), `cmd/gromit/run2.go` (router construction). No changes to v1 packages.

### Boundaries and Contracts

**Router contract:** `Select(phase, tier) -> (LLMProvider, model, error)`. Router owns tier-to-model resolution and provider selection. Stages never call the router directly.

**Stage contract:** Unchanged `Stage.Run(ctx, *StageRequest) -> (*StageResult, error)`. Stages use `req.Provider` if set, otherwise fall back to constructed provider. `req.Model` is populated by the loop.

**Loop as routing seam:** The bead loop and spec loop call `Router.Select()`, populate `StageRequest.Model` and `StageRequest.Provider`, then call `stage.Run()`. The loop determines starting tier from `methodology.phase_models[phase]` and handles escalation by incrementing tier on retry.

**Remediation contract:** Expanded to `Run(ctx, specID, worktree string) error`. Worktree propagates into the Request template for all remediation stages.

**Prompt assembler contract:** `Assemble(phase string, fileCount int) string`. Phase drives rules filtering and cap selection. File count drives scope adjustment. Returns shaped prompt within budget. Produces a `ShapeReport` as a side effect for observability.

**AdapterSet:** Unchanged. The router is a separate concern passed to the loop, not an adapter.

## Test Strategy

### Multi-Provider Routing
- Unit tests for router: ratio balancing produces correct provider selection given invocation counts
- Unit tests for router: phase preferences override ratio balancing
- Unit tests for router: fallback marks provider unavailable, selects alternative, re-enables after cooldown
- Unit tests for tier resolution: `methodology.phase_models` overrides read correctly per phase
- Unit tests for tier resolution: abstract tier maps to correct provider-specific model name

### Tier-Aware Model Selection
- Integration tests for bead loop routing: mock router + stages, verify correct provider/model in StageRequest per phase
- Integration tests for escalation: verify tier bumps on retry, verify router re-called with higher tier
- Integration test: build phase starts at "low" tier when `methodology.phase_models.build` is "low"

### Resume
- Unit tests for gap analysis: given file diff and bead-to-file mapping, correct beads flagged
- Unit tests for gap analysis: beads with no file overlap are not flagged
- Integration tests for resume: existing plan + beads -> gap analysis -> selective revalidation -> correct bead list
- Integration test: remediation receives correct worktree path and Request template is populated

### Topological Ordering
- Unit tests for topological sort: DAG ordering respects DependsOn edges
- Unit tests for topological sort: cycle detection returns error
- Unit tests for topological sort: beads with no dependencies come first, tie-breaking is stable
- Integration tests for gate re-queue: blocked bead deferred to end of pass
- Integration test: blocked bead proceeds after dependency completes in same pass
- Integration test: all-blocked pass terminates with descriptive error

### Prompt Budget Shaping
- Unit tests for phase filtering of RULES.md: correct sections retained, cap respected
- Unit tests for CLAUDE.md scoping: given bead files, only relevant package sections included
- Unit tests for budget shaping: total output within cap, layers trimmed in priority order
- Unit tests for scope adjustment: small bead (1-2 files) gets 50% budget, large bead gets 100%
- Unit tests for ShapeReport: reports what was trimmed and by how much
- Integration test: assemble a full build prompt, verify size is within expected range vs v1 baseline

## Research & Context

### Current V2 State

The v2 spec loop (`internal/v2/loop/`) was built as a separate pipeline from v1 (`internal/runner/`). It uses a simplified adapter architecture (`internal/v2/adapter/`) where a single `LLMAdapter` wraps a hardcoded Claude provider. The v1 runner has full multi-provider routing (`internal/provider/router.go`), tier selection (`internal/config/config_accessors.go`), escalation (`internal/runner/escalation/`), and prompt budget shaping (`internal/prompt/renderer_budget.go`), none of which are wired into v2.

### V1 Capabilities Being Ported

- **Router:** `internal/provider/router.go` -- 3-layer selection (phase preferences, ratio balancing, fallback with cooldown)
- **Tier selection:** `internal/config/config_accessors.go` -- `BuildTierForStrategy()`, `PhaseModelTier()`, `SelectInitialTierForComplexity()`
- **Prompt shaping:** `internal/prompt/renderer_budget.go` -- `ShapeContextForBudget()`, `shapeBuildContext()`, `scopeAdjustedBudget()`
- **Rules filtering:** `internal/prompt/renderer_context.go` -- `LoadRulesForPhase()` with `rulesPhaseMaxChars` caps
- **Context scoping:** `internal/prompt/renderer_context.go` -- `applyScopedClaudeContext()`

### Quantitative Impact

Current v2 build prompt: ~27 KB base (RULES.md 23.6 KB unfiltered + CLAUDE.md 1.1 KB + fragment 2.2 KB).

After shaping: ~5-6 KB for small beads, ~8-10 KB for medium, ~11-13 KB for large. Typical 4-5x reduction for decomposed beads which are mostly small.

### Related Specs

- `multi-provider-routing` -- V1 multi-provider routing spec (accepted). This spec ports the same capabilities into v2 natively.
- `immutable-pipeline` -- V2 pipeline spec this depends on for worktree branch lifecycle, stage commits, and event logging.
- `complexity-based-routing` -- V1 complexity-based tier selection. V2 tier resolution draws from the same config knobs.
