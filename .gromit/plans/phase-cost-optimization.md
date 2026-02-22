---
created: 2026-02-22T00:00:00Z
decomposed: true
decomposed_at: "2026-02-22T17:18:05Z"
id: phase-cost-optimization
source_spec: phase-cost-optimization
---

# Phase-Aware Cost Optimization Implementation Plan

**Goal:** Reduce per-bead invocation cost by selecting models per-phase, adding a single-pass build strategy, tiering reviews, and sending each phase only the context it needs.

**Architecture:** Extend config with `build_strategy` and `phase_models` fields, add hardcoded `PhaseContextProfile` to the prompt package, wire per-phase tier resolution into callbacks/orchestrator, and add a single-pass build path alongside the existing TDD path.

**Tech Stack:** Go, YAML config, Go templates

**Spec:** `.gromit/specs/phase-cost-optimization.md`

---

## Architecture

### Key Components

1. **Config Layer** (`internal/config/`): `PhaseModelsConfig` struct with per-phase tier overrides. `BuildStrategy` field on `MethodologyConfig`. `Tier` field on `ReviewConfig` replacing `Model`. `PhaseModelTier(phase, beadTier)` accessor that resolves phase tier with bead-tier fallback.

2. **Prompt Layer** (`internal/prompt/`): `PhaseContextProfile` type mapping phase names to included context sections. Hardcoded default profiles matching the spec table. `BuildContext()` accepts phase parameter and zeros excluded fields before budget shaping.

3. **Callback/Orchestrator Layer** (`internal/runner/`): Per-phase tier resolution via `PhaseModelTier()` before each invocation. Correct phase name passed to `LoadRulesForPhase()`. Tier pointer seeded per-phase.

4. **Build Strategy Path** (`internal/runner/`): Single-pass path: build → validate → refactor → validate → review. Strategy read from config + bead label override. TDD path unchanged but opt-in.

5. **Review Layer** (`internal/pipeline/review/`, `internal/runner/reviewpkg/`): Tier-based resolution via router instead of model string.

### Integration Points

- Router already accepts `(phase, tier)` — no router changes needed
- Phase metrics already record model/tier per invocation — no metrics changes needed
- Escalation already per-phase via tier pointer — no escalation changes needed
- Budget shaping already accepts phase — runs after phase scoping as belt-and-suspenders

### Tradeoffs

- **Hardcoded profiles over configurable**: Phases have inherent context needs. Budget shaping is the configurable escape hatch.
- **Modify BuildContext() over new function**: Adding phase parameter keeps one entry point instead of duplicating logic.
- **Auto-map legacy fields over breaking change**: Consistent with how config handles legacy model names elsewhere.

---

## Test Strategy

### Unit Tests

- `PhaseModelsConfig` YAML parsing: all fields set, partial fields, empty config
- `PhaseModelTier()` accessor: returns phase override when set, returns bead tier when empty, handles unknown phase
- `BuildStrategy` parsing: "single_pass", "tdd", default when absent
- `ReviewConfig.Tier` parsing: new field, legacy `Model` auto-mapping, `match_build_model` deprecation warning
- `PhaseContextProfile`: each phase gets exactly the sections listed in spec table
- `ApplyPhaseProfile()`: excluded fields zeroed, included fields preserved
- `BuildContext(phase)`: red phase excludes learnings/ClaudeMD, build phase includes them
- Build strategy label parsing from bead labels

### Integration Tests

- Per-phase tier resolution in TDD callbacks: red gets low tier, green gets bead tier
- Single-pass build path: correct invocation sequence (build → validate → refactor → validate)
- Review tier resolution: regular review at low, thorough at high
- Backward compatibility: config without new fields behaves as single-pass with bead-tier everywhere

### Mocking Strategy

- Config tests: direct struct construction, YAML unmarshal
- Prompt tests: fixture-based context structs
- Callback tests: existing FnField mock pattern
- Build path tests: mock InvokeFn/ValidateFn per existing pattern

### Coverage Goals

- All phase × tier combinations for PhaseModelTier
- All 6 phases for context profile application
- Both build strategies exercised end-to-end through process_methodology
- Legacy config migration (model→tier, match_build_model deprecation)

---

## Implementation Tasks

### Task 1: Add PhaseModelsConfig and BuildStrategy to config types

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `PhaseModelsConfig` struct with `Decompose`, `Build`, `Red`, `Green`, `Refactor` string fields (yaml-tagged). Add `BuildStrategy string` and `PhaseModels PhaseModelsConfig` fields to `MethodologyConfig`. Add `Refactor bool` field to `MethodologyConfig` if not present. In defaults, set `BuildStrategy` to `"single_pass"`, `Refactor` to `true`, `PhaseModels.Red` to `"low"`, `PhaseModels.Refactor` to `"low"`, `PhaseModels.Decompose` to `"medium"`, others to `""` (inherit). Normalize nil fields for the new struct.

**Acceptance Criteria:**
- YAML with `methodology.build_strategy: tdd` and `methodology.phase_models.red: low` parses correctly
- Config without `build_strategy` defaults to `"single_pass"`
- Config without `phase_models` defaults to red=low, refactor=low, decompose=medium, build/green=""

**Dependencies:** None

### Task 2: Add PhaseModelTier() accessor

**Files:**
- Modify: `internal/config/config_accessors.go`
- Test: `internal/config/config_accessors_test.go`

**What to Do:**
Add `PhaseModelTier(phase string, beadTier string) string` method on `*Config`. Lookup phase in `Methodology.PhaseModels` — if the matching field is non-empty, return it; otherwise return `beadTier`. Map phase names to struct fields: "decompose"→Decompose, "build"→Build, "red"→Red, "green"→Green, "refactor"→Refactor. Unknown phases return beadTier.

**Acceptance Criteria:**
- `PhaseModelTier("red", "high")` returns `"low"` when red is configured as low
- `PhaseModelTier("green", "high")` returns `"high"` when green is empty (inherits bead tier)
- `PhaseModelTier("unknown", "medium")` returns `"medium"`

**Dependencies:** Task 1

### Task 3: Change ReviewConfig to use Tier with backward compatibility

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config_defaults.go`
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `Tier string` field to `ReviewConfig` (yaml `tier`). Add `Tier string` field to `ThoroughReviewConfig` (yaml `tier`). In defaults, set `Review.Tier` to `"low"` and `Thorough.Tier` to `"high"`. In `NormalizeNilFields` or a post-load hook: if `Review.Model` is set and `Review.Tier` is empty, auto-map via `TierFromLegacyModel(Review.Model)`. Same for thorough. If `MatchBuildModel` is non-nil, log a deprecation warning and ignore it. Keep `Model` and `MatchBuildModel` fields for YAML parsing but mark them as legacy.

**Acceptance Criteria:**
- Config with `review.tier: low` parses and uses `low`
- Config with legacy `review.model: haiku` auto-maps to `review.tier: low`
- Config with `match_build_model: true` produces a deprecation warning in normalize output

**Dependencies:** None

### Task 4: Add PhaseContextProfile type and default profiles

**Files:**
- Create: `internal/prompt/phase_profile.go`
- Create: `internal/prompt/phase_profile_test.go`

**What to Do:**
Define `PhaseContextProfile` as a map from phase name to a set of included context section names. Define `DefaultPhaseProfiles()` returning profiles matching the spec table:

| Phase | Sections |
|-------|----------|
| decompose | Spec(full), ClaudeMD(scoped) |
| red | Spec(excerpt), TargetCriterion, CoverageState, Rules(phase-filtered) |
| build | Spec(excerpt), ClaudeMD(scoped), Rules(phase-filtered), ConfirmedLearnings, ValidationFailures, CoverageState, TargetCriterion, PrevFailure, SiblingTouchedPackages |
| green | Same as build |
| refactor | Rules(phase-filtered), ValidationFailures |
| review | Diff, Rules(phase-filtered) |
| thorough_review | Diff, Rules(full), ClaudeMD(scoped), ConfirmedLearnings |

Add `ApplyPhaseProfile(ctx *Context, phase string)` that zeros excluded fields on the Context struct. Add `ApplyReviewPhaseProfile(ctx *ReviewContext, phase string)` for review contexts. Handle "excerpt" vs "full" spec distinction with a `SpecExcerpt` field or by truncating spec to the relevant criterion section.

**Acceptance Criteria:**
- `ApplyPhaseProfile(ctx, "red")` zeros ClaudeMD, ConfirmedLearnings, RecentLearnings, PrevFailure, SiblingTouchedPackages, ValidationFailures
- `ApplyPhaseProfile(ctx, "build")` preserves ConfirmedLearnings, ValidationFailures, CoverageState, PrevFailure, SiblingTouchedPackages
- `ApplyPhaseProfile(ctx, "refactor")` zeros Spec, ClaudeMD, ConfirmedLearnings, CoverageState, TargetCriterion

**Dependencies:** None

### Task 5: Integrate phase-scoped context into BuildContext

**Files:**
- Modify: `internal/prompt/renderer_context.go`
- Test: `internal/prompt/prompt_test.go` or `internal/prompt/renderer_context_test.go`

**What to Do:**
Add `phase string` parameter to `BuildContext()`. After loading all context sections (existing logic), call `ApplyPhaseProfile(ctx, phase)` to zero excluded fields. This runs before budget shaping. Update all callers of `BuildContext()` to pass the appropriate phase name. For callers that don't know the phase (or for backward compatibility), accept empty string to mean "no profile applied" (all sections included).

**Acceptance Criteria:**
- `BuildContext(bead, parent, iter, model, "red")` returns context with zeroed ClaudeMD and learnings
- `BuildContext(bead, parent, iter, model, "")` returns full context (backward compatible)
- Budget shaping still runs after phase scoping

**Dependencies:** Task 4

### Task 6: Wire per-phase tier resolution into TDD callbacks

**Files:**
- Modify: `internal/runner/callbacks_tdd.go`
- Test: `internal/runner/callbacks_tdd_test.go`

**What to Do:**
In `buildRenderRedFn`, resolve `cfg.PhaseModelTier("red", bc.Tier)` and set it on bc before invoking. Pass `"red"` to `LoadRulesForPhase()` (currently passes `"build"`). In `buildRenderGreenFn`, resolve `cfg.PhaseModelTier("green", bc.Tier)` and pass `"green"` to `LoadRulesForPhase()` (currently passes `"build"`). In `buildRunRefactorFn`, resolve `cfg.PhaseModelTier("refactor", bc.Tier)`. The orchestrator's `invokeWithRetryAndEscalation` already accepts a tier pointer — seed it with the phase-resolved tier. Phase metrics (`appendTDDPhaseMetric`) already capture `bc.Tier` per phase.

**Acceptance Criteria:**
- Red phase invocation uses tier from `PhaseModelTier("red", beadTier)`, not raw bead tier
- Green phase invocation uses tier from `PhaseModelTier("green", beadTier)`
- `LoadRulesForPhase` is called with "red" for red phase and "green" for green phase (not "build" for both)

**Dependencies:** Task 2

### Task 7: Add single-pass build path

**Files:**
- Modify: `internal/runner/process_methodology.go`
- Test: `internal/runner/process_methodology_test.go`

**What to Do:**
Read build strategy: check bead labels for `build_strategy:tdd` or `build_strategy:single_pass` override, else use `cfg.Methodology.BuildStrategy`. When strategy is `single_pass`: invoke one build phase at `PhaseModelTier("build", beadTier)` with phase context profile "build", then validate, then (if `cfg.Methodology.Refactor`) invoke refactor at `PhaseModelTier("refactor", beadTier)` with "refactor" profile, then validate again. The existing TDD path runs when strategy is `tdd`. Reuse existing invoke/validate/refactor infrastructure — this is a different orchestration of the same building blocks.

**Acceptance Criteria:**
- Config with `build_strategy: single_pass` executes: build → validate → refactor → validate (not red/green cycles)
- Bead label `build_strategy:tdd` overrides global `single_pass` default
- `methodology.refactor: false` skips the refactor phase in single-pass

**Dependencies:** Task 2, Task 5

**Notes:** This is the largest task and may decompose into 2-3 beads. The build invocation reuses the same `InvokeFn` pattern as TDD but with a single-shot build prompt instead of red/green prompts. May need a new template (PROMPT_single_pass_build.md) or reuse the TDD green template with adjusted context.

### Task 8: Wire tier-based review into review stages

**Files:**
- Modify: `internal/pipeline/review/review.go`
- Modify: `internal/runner/reviewpkg/reviewer.go`
- Test: existing review test files

**What to Do:**
In the pipeline review stage, replace `in.Config.Review.Model` with `in.Config.Review.Tier` when selecting the model via router. The router's `Select(phase, tier)` already handles tier→model resolution. Same change for thorough review using `in.Config.Review.Thorough.Tier`. In the runner-level reviewpkg, make the equivalent change. Apply "review" or "thorough_review" phase context profiles to review context structs.

**Acceptance Criteria:**
- Regular review invokes at `ReviewConfig.Tier` (default: low) not `ReviewConfig.Model`
- Thorough review invokes at `ReviewConfig.Thorough.Tier` (default: high)
- Review context contains only Diff and phase-filtered Rules per profile

**Dependencies:** Task 3, Task 4

### Task 9: Update RULES.md phase annotations

**Files:**
- Modify: `.gromit/RULES.md`

**What to Do:**
Review each section in RULES.md and add/update `<!-- phases: ... -->` annotations per the context profile table. Sections relevant to build/green phases get `build, green`. Sections relevant to red get `red`. Sections relevant to refactor get `refactor`. Sections relevant to review get `review`. Sections relevant to all phases get no annotation (included everywhere by default). Sections relevant to decompose get `decompose`. Verify `LoadRulesForPhase()` returns correct filtered content for each phase.

**Acceptance Criteria:**
- `LoadRulesForPhase("red")` returns only sections annotated with `red`
- `LoadRulesForPhase("refactor")` returns only sections annotated with `refactor`
- Sections without annotations are included in all phases (backward compatible)

**Dependencies:** None

### Task 10: Build strategy label inheritance in decomposition

**Files:**
- Modify: wherever bead decomposition creates sub-beads (likely `internal/pipeline/decompose/` or equivalent)
- Test: decomposition test file

**What to Do:**
When decomposing a bead into sub-beads, copy the parent's `build_strategy:X` label onto each sub-bead. If the parent has no build_strategy label, sub-beads get none (inherit global config). This ensures that a bead marked `build_strategy:tdd` produces sub-beads that also use TDD.

**Acceptance Criteria:**
- Parent bead with `build_strategy:tdd` label produces sub-beads with `build_strategy:tdd` label
- Parent bead with no build_strategy label produces sub-beads with no build_strategy label

**Dependencies:** Task 1

---

## Notes

- **Router, escalation, and phase metrics need no changes.** The existing infrastructure already supports per-phase tier, per-phase escalation, and per-phase metrics recording. This plan only touches config, prompt, callbacks, and orchestration.
- **Single-pass build template.** Task 7 may need a new prompt template (PROMPT_single_pass_build.md) or may reuse the existing TDD green template. Decide during implementation based on how different the prompt needs are.
- **Spec excerpt vs full spec.** The context profile distinguishes "full" spec (decompose) from "excerpt" (build/red/green). The excerpt is the spec section relevant to the current criterion. This may require a `SpecExcerpt` field on Context or a trimming function. Decide during Task 4 implementation.
- **Backward compatibility is critical.** Configs without any new fields must behave as single-pass with bead-tier everywhere and review at current defaults. All legacy field auto-mapping must be tested.
- **CLAUDE.md scoping.** The spec mentions investigating `--no-project-context` to suppress automatic CLAUDE.md loading. This is a research item that can be deferred — for now, phases that don't need CLAUDE.md simply get it zeroed in their context profile.
