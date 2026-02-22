---
id: phase-cost-optimization
source_ideas: []
created: 2026-02-22
supersedes:
  - reduce-iteration-cost (Lever B only)
  - phase-isolated-methodology-contexts (Minimal Phase Prompts section only)
---

# Phase-Aware Cost Optimization

## Specification

Gromit currently uses one model tier per bead for every phase and always runs multi-invocation TDD cycles. This wastes money in two ways: cheap phases use expensive models, and well-decomposed beads pay TDD overhead they don't need. A single haiku invocation succeeds at near-100% on well-scoped beads (per `cost-optimized-routing` metrics), yet the TDD path forces 3-5 invocations with fresh-context overhead on every one.

This spec introduces four changes:

1. **Build strategy toggle** — choose between single-pass and TDD per bead
2. **Per-phase model tier selection** for TDD cycles
3. **Two-tier review** with cheap regular reviews and configurable thorough reviews
4. **Phase-scoped context profiles** that send each phase only the context it needs

All model references use abstract tiers (`low`, `medium`, `high`), never provider-specific names. The toggle is designed as a simple enum now, with room to evolve into richer automatic routing strategies later.

### 1. Build Strategy Toggle

A `build_strategy` field selects how beads are built. The refactor phase fires regardless of strategy.

```yaml
methodology:
  build_strategy: single_pass   # single_pass (default) or tdd
  refactor: true                # separate refactor phase, independent of strategy
  phase_models:
    build: ""                   # single-pass build tier (inherit bead tier)
    red: low                    # TDD only
    green: ""                   # TDD only (inherit bead tier)
    refactor: low               # both strategies
```

**Strategies:**

**`single_pass`** (default) — One build invocation at the bead's selected tier, then validate, then refactor, then validate again, then review. Best for well-decomposed beads where the spec is clear and the scope is narrow. This is the cost-effective default because well-scoped beads don't benefit from TDD's incremental approach, and a single invocation avoids fresh-context overhead.

**`tdd`** — Red/green/refactor cycles with separate invocations per phase. Best for complex beads where incremental test-first building prevents going down wrong paths. Opt in via config or per-bead label (`build_strategy:tdd`).

**Behavior:**

- Default is `single_pass`. Existing configs without `build_strategy` get single-pass behavior.
- Per-bead label override: `build_strategy:tdd` or `build_strategy:single_pass` on a bead overrides the global default.
- When a bead is decomposed, sub-beads inherit the parent's `build_strategy` label.
- The refactor phase is independent: `methodology.refactor: true` (default) fires a separate refactor invocation after validation passes, regardless of build strategy. LLMs skip refactoring in single-pass builds just as they do in TDD; a separate invocation mechanically enforces it.

**Single-pass flow:**

1. Decompose (if bead is too broad, split into well-scoped sub-beads at `medium` tier)
2. Build (one invocation, bead tier, green context profile)
3. Validate
4. Refactor (separate invocation, refactor tier)
5. Validate
6. Review

**TDD flow:**

1. Decompose (if bead is too broad, split into well-scoped sub-beads at `medium` tier)
2. Red (write failing test, red tier)
3. Validate (expect fail)
4. Green (implement to pass, green tier)
5. Validate (expect pass)
6. Repeat for each criterion
7. Refactor (separate invocation, refactor tier)
8. Validate
9. Review

**Decomposition is critical to single-pass success.** Single-pass works because each bead is narrow enough for one invocation to handle completely. The decompose step (existing Gate stage) ensures beads are well-scoped before build begins. If a bead is too broad, decomposition splits it into sub-beads that each target a single concern. The decompose phase uses `medium` tier because decomposition quality is leverage — one bad split wastes N cheap build attempts.

**Future direction:** The `build_strategy` field is a simple enum today. It can evolve into a richer routing system (e.g., automatic strategy selection based on bead complexity, scope metrics, or failure history) without breaking existing configs. New strategies can be added as new enum values.

### 2. Per-Phase Model Selection

Each phase can specify its own tier. Phases that omit a tier inherit the bead's selected tier.

```yaml
methodology:
  phase_models:
    decompose: medium  # quality decomposition is leverage
    build: ""          # single-pass build tier (inherit bead tier)
    red: low           # TDD: write one failing test from spec
    green: ""          # TDD: inherit bead tier
    refactor: low      # both: cleanup with passing tests as safety net
```

**Behavior:**

- `phase_models` is optional. When absent, all phases use the bead tier (backward compatible).
- Empty string or missing key means inherit bead tier.
- Values are tier names: `low`, `medium`, `high`.
- The `build` key applies to single-pass strategy. The `red` and `green` keys apply to TDD strategy only. The `decompose` key applies to both.
- Escalation works per-phase. If red fails at `low`, it escalates through `medium` then `high` using the existing `invokeWithRetryAndEscalation` mechanism.
- Phase metrics already record model and tier per invocation; cost tracking requires no changes.

**Defaults:**

| Phase | Default Tier | Rationale |
|-------|-------------|-----------|
| Decompose | `medium` | Decomposition quality is leverage; a bad split wastes N build attempts |
| Build | bead tier | Single-pass must handle the full problem in one shot |
| Red | `low` | Writing a single failing test from a spec excerpt is mechanical |
| Green | bead tier | Implementation is the hardest reasoning task |
| Refactor | `low` | Tests are passing; the task is cleanup and naming |

### 2. Two-Tier Review

Regular review defaults to a cheap tier. Thorough review runs periodically at a higher tier.

```yaml
review:
  enabled: true
  tier: low
  thorough:
    enabled: true
    tier: high
    every_n_iterations: 5
    on_epic_complete: true
```

**Behavior:**

- `review.tier` replaces `review.model`. Defaults to `low`.
- `review.thorough.tier` replaces `review.thorough.model`. Defaults to `high`.
- `match_build_model` is removed. Review tier is now explicit.
- Thorough review frequency and epic-complete trigger remain configurable.

**Backward compatibility:** If a config sets `review.model` (legacy) without `review.tier`, auto-map the model name to a tier via `TierFromLegacyModel()`. Same for `review.thorough.model`.

### 3. Phase-Scoped Context Profiles

Each phase declares which context sections it needs. This is hardcoded in the prompt builder, not configurable. The existing budget shaping still applies on top of phase profiles as a secondary constraint.

**Profiles:**

The `Build` column applies to single-pass strategy and uses the same profile as `Green` — both are implementation phases that need the richest context.

| Section | Decompose | Red | Build/Green | Refactor | Review | Thorough Review |
|---------|-----------|-----|-------------|----------|--------|-----------------|
| Spec | full | excerpt | excerpt | no | no | no |
| ClaudeMD | scoped | no | scoped | no | no | scoped |
| Rules | no | phase-filtered | phase-filtered | phase-filtered | phase-filtered | full |
| ConfirmedLearnings | no | no | yes | no | no | yes |
| RecentLearnings | no | no | no | no | no | no |
| ValidationFailures | no | no | yes | yes | no | no |
| CoverageState | no | yes | yes | no | no | no |
| TargetCriterion | no | yes | yes | no | no | no |
| PrevFailure | no | no | yes | no | no | no |
| SiblingTouchedPackages | no | no | yes | no | no | no |
| Diff | no | no | no | no | yes | yes |

**Rationale per phase:**

- **Decompose** needs the full spec and scoped project architecture to make good splitting decisions. It does not need rules, learnings, or failure context — it is planning, not implementing.
- **Red** needs only the spec excerpt and target criterion to write a failing test. Full project context distracts from the narrow task.
- **Build/Green** gets the richest context because implementation is the hardest phase. It needs learnings, failure history, sibling context, and scoped project architecture.
- **Refactor** needs only rules (for style guidance) and validation failures (to avoid known issues). Tests are passing; the code speaks for itself.
- **Review** needs the diff and rules to judge output. It does not need implementation context.
- **Thorough review** adds learnings and full rules for deeper architectural analysis.

**RULES.md phase filtering:** Rules sections are tagged with phase annotations (e.g., `<!-- phases: build, review -->`). The renderer filters to sections matching the current phase. The full RULES.md remains the single source of truth for humans. This approach (from `reduce-iteration-cost` Lever B) avoids maintaining separate rules files.

**CLAUDE.md scoping:** Green and thorough review use scoped CLAUDE.md (relevant sections extracted based on touched packages). All other phases omit it entirely. Investigate whether `--no-project-context` can suppress automatic CLAUDE.md loading for phases that provide their own full context via prompt.

**Implementation:**

- New `PhaseContextProfile` type that maps phase names to sets of included context sections.
- `prompt.BuildContext()` accepts a phase name and consults the profile to skip excluded sections.
- Existing budget shaping runs after phase scoping as a belt-and-suspenders measure.
- Existing template registration pattern is preserved; templates still receive a `Context` struct, but excluded fields are zero-valued.

## Acceptance Criteria

### Build Strategy Toggle
- `methodology.build_strategy` config field accepts `single_pass` and `tdd`; defaults to `single_pass`
- Per-bead label override: `build_strategy:tdd` or `build_strategy:single_pass` overrides the global default
- Sub-beads inherit the parent's `build_strategy` label during decomposition
- Single-pass strategy executes: build → validate → refactor → validate → review
- TDD strategy executes: red/green cycles → refactor → validate → review
- `methodology.refactor` boolean (default `true`) controls whether the refactor phase fires; independent of strategy
- Decomposition runs before build and targets well-scoped beads; the decompose phase uses `medium` tier to produce quality splits

### Per-Phase Model Selection
- `methodology.phase_models` config is parsed with `build`, `red`, `green`, and `refactor` keys accepting tier names
- `build` key applies to single-pass strategy; `red` and `green` keys apply to TDD strategy only
- Omitted phase keys inherit the bead's selected tier
- Each phase invokes Claude at its configured tier
- Escalation works independently per phase (red can escalate without affecting green's starting tier)
- Phase metrics record the actual tier and model used per phase invocation

### Two-Tier Review
- `review.tier` config field accepts tier names and defaults to `low`
- `review.thorough.tier` config field accepts tier names and defaults to `high`
- Legacy `review.model` and `review.thorough.model` fields auto-map to tiers via `TierFromLegacyModel()`
- `match_build_model` field is removed; configs using it produce a deprecation warning
- Thorough review triggers every N iterations (configurable) and on epic complete

### Phase-Scoped Context
- Decompose phase prompts contain: full spec, scoped CLAUDE.md
- Red phase prompts contain only: spec excerpt, target criterion, coverage state, phase-filtered rules
- Build/green phase prompts contain: spec excerpt, scoped CLAUDE.md, phase-filtered rules, confirmed learnings, validation failures, coverage state, target criterion, previous failure, sibling touched packages
- Refactor phase prompts contain only: phase-filtered rules, validation failures
- Review prompts contain only: diff, phase-filtered rules
- Thorough review prompts contain: diff, full rules, scoped CLAUDE.md, confirmed learnings
- RULES.md supports phase annotations; the renderer filters sections by phase
- Existing budget shaping still applies after phase scoping
- Backward compatible: configs without `build_strategy`, `phase_models`, or `review.tier` default to single-pass with current bead-tier behavior

## Decisions

1. **Single-pass as default, TDD as opt-in.** Well-decomposed beads succeed at near-100% with a single cheap invocation. TDD's multi-invocation overhead (fresh context per phase, re-establishing understanding) costs more than it saves for narrow beads. TDD remains available for complex beads where incremental test-first building genuinely helps. The toggle is a simple enum now, designed to evolve into richer automatic routing later.

2. **Decomposition at medium tier is non-negotiable.** Single-pass works only if beads are well-scoped. Decomposition quality is leverage: one bad split wastes N cheap build attempts. Paying for `medium` tier decomposition saves money downstream. This applies to both strategies.

3. **Separate refactor phase for both strategies.** LLMs skip refactoring whether building in single-pass or TDD. A separate invocation mechanically enforces it. The refactor toggle is independent of build strategy because the problem (LLMs don't refactor unprompted) is universal.

4. **Tier names, not model names.** All config uses `low`/`medium`/`high`. Provider-specific model names are a deployment concern, not a workflow concern. The existing `TierToLegacyModel()` bridge handles resolution.

5. **Green/build inherits bead tier by default.** Implementation is the hardest phase. Defaulting it to the bead's priority-selected tier ensures P0 beads still get opus for implementation while red, refactor, and review go cheap.

6. **Hardcoded context profiles, not configurable.** What context a phase needs is inherent to the phase's purpose, not a user preference. Hardcoding avoids config sprawl and prevents users from accidentally starving a phase of necessary context. The budget shaping system remains as the configurable escape hatch.

7. **Regular review at low tier.** Regular review catches obvious issues (missing error handling, naming problems, pattern violations). Thorough review at high tier handles architectural judgment. The two-tier system lets us be cheap on the frequent check and thorough on the periodic one.

8. **Phase annotations in RULES.md over separate files.** A single rules file avoids drift between copies. Phase annotations are parsed by the renderer and invisible to humans reading the file. If annotations prove too noisy, split into files and concatenate for display.

9. **Backward-compatible migration.** Legacy `model` fields auto-map to tiers. `match_build_model` produces a deprecation warning rather than a hard error. Configs without new fields or `build_strategy` behave as single-pass (current default minus TDD).

## Supersedes

### reduce-iteration-cost — Lever B (Trim Static Context)

This spec fully replaces Lever B. Phase-scoped context profiles are a more complete implementation of the same goal: each phase gets only the context it needs. The RULES.md phase-annotation approach originates from Lever B and is preserved here.

Levers A (validation failure injection, self-check instructions) and C (learnings filtering, `max_learning_chars` cap) remain in `reduce-iteration-cost` and are unaffected.

### phase-isolated-methodology-contexts — Minimal Phase Prompts

This spec replaces the "Minimal Phase Prompts" section with explicit context profiles per phase. The concrete profile table here supersedes the general guidance in that spec.

The context isolation concerns (per-phase timeouts, cancellation boundaries, `ParentCtx` derivation, error semantics) remain in `phase-isolated-methodology-contexts` and are unaffected. Those are orthogonal: isolation controls *when* a phase stops, this spec controls *what* a phase sees.

## Research & Context

### Current State

- Model selection: `SelectTier()` in `internal/config/config_accessors.go:82-110` picks one tier per bead based on priority + label overrides.
- TDD phases: `internal/runner/tdd/orchestrator.go` runs red/green/refactor with injected `InvokeFn` that receives a single tier pointer.
- Escalation: `invokeWithRetryAndEscalation` in `internal/runner/tdd/orchestrator.go:119-143` retries then escalates the tier pointer.
- Review config: `ReviewConfig` in `internal/config/config_types.go:239-253` uses `Model string` and `MatchBuildModel *bool`.
- Budget shaping: `internal/prompt/budget.go` trims context sections in priority order when over budget.
- Phase metrics: `appendTDDPhaseMetric` in `internal/runner/callbacks_tdd.go:225-262` records model/tier per phase.

### Key Files to Modify

- `internal/config/config_types.go` — Add `PhaseModels` struct to `MethodologyConfig`; change `ReviewConfig.Model` to `ReviewConfig.Tier`
- `internal/config/config_defaults.go` — Default phase_models (red=low, refactor=low); default review.tier=low, review.thorough.tier=high
- `internal/config/config_accessors.go` — Add `PhaseModelTier(phase, beadTier)` accessor
- `internal/runner/callbacks_tdd.go` — Resolve per-phase tier before passing to `InvokeFn`
- `internal/runner/tdd/orchestrator.go` — Seed tier pointer per-phase from config instead of once per cycle
- `internal/prompt/prompt.go` — Add `PhaseContextProfile` type; modify `BuildContext()` to accept phase name
- `internal/prompt/budget.go` — Phase scoping runs before budget shaping
- `internal/pipeline/review/review.go` — Resolve review tier instead of review model
- `.gromit/RULES.md` — Add phase annotations to sections

### Related Specs

- `cost-optimized-routing` — Proposes a new `cost_optimized` routing strategy (haiku-first, decompose-on-failure). Complementary: that spec changes *which* bead gets which base tier; this spec changes *which phase within a bead* gets which tier. Both emphasize decomposition quality as leverage.
- `tdd-methodology` — Defines the TDD workflow. Decision 2 ("same model for all phases") is superseded by per-phase model selection. The assumption that TDD is the primary build approach is superseded by the build strategy toggle — TDD becomes opt-in.
- `reduce-iteration-cost` — Levers A and C remain active. Lever B is superseded.
- `phase-isolated-methodology-contexts` — Context isolation (timeouts, cancellation) remains active. Minimal phase prompts section is superseded.
