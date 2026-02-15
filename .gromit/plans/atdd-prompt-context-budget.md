---
created: 2026-02-15T00:00:00Z
decomposed: true
decomposed_at: "2026-02-15T20:28:48Z"
id: atdd-prompt-context-budget
source_spec: atdd-prompt-context-budget
---

# ATDD Prompt Context Budget Implementation Plan

**Goal:** Add deterministic, configurable ATDD acceptance-prompt budgeting that materially reduces prompt size without losing required task coverage.

**Architecture:** Keep `BuildContext` unchanged for non-ATDD paths, and add an ATDD-only shaping step before acceptance prompt rendering that applies inclusion toggles, learnings caps, and deterministic trim actions with observability logs.

**Tech Stack:** Go, YAML config, Go `text/template` prompt rendering, existing runner/methodology logging and test framework.

**Spec:** `.gromit/specs/atdd-prompt-context-budget.md`

---

## Architecture Proposal

**Overview:**
Add an ATDD-only prompt shaping layer that derives a budgeted acceptance context from `prompt.Context`, applies deterministic trimming, and logs section-level prompt metrics before invoking acceptance generation.

**Key Components:**
1. **ATDD prompt config (`internal/config/config.go`)**: Add a nested ATDD prompt config under `methodology` to control inclusion and char budgets.
2. **ATDD context shaper (`internal/prompt/prompt.go`)**: Add deterministic shaping logic that copies `Context`, applies toggles/caps, trims by ordered actions, and guarantees bead identity fields remain.
3. **ATDD rules subset loader (`internal/prompt/prompt.go`)**: Add an ATDD-focused rules extractor (from `RULES.md`) and use it during trim fallback.
4. **ATDD render path wiring (`internal/runner/callbacks.go` + `internal/runner/methodology/executor.go`)**: Switch acceptance rendering to use shaped context and emit per-attempt prompt stats in normal logs.
5. **Template compatibility (`.gromit/templates/PROMPT_acceptance_tests.md`)**: Keep existing fields, but ensure behavior remains correct when sections are intentionally omitted/truncated.

**Integration Points:**
- Keep `BuildContext` unchanged for build/review phases; apply shaping only on ATDD acceptance prompt generation.
- Reuse existing `RenderAcceptanceTests` template API to avoid broad interface churn.
- Use existing `Executor.log(...)` and runner logging so metrics appear in normal run output.

**Data Flow:**
1. `processBead` builds full `bc.PromptCtx` as today.
2. `RunAcceptanceTests` calls render callback.
3. Render callback shapes `bc.PromptCtx` using ATDD prompt config.
4. Shaped context is rendered via `PROMPT_acceptance_tests.md`.
5. Before/after chars, section sizes, and trim actions are logged once per acceptance invocation.
6. Prompt is passed to provider invocation unchanged after shaping.

**Files to Modify:**
- `internal/config/config.go` - add ATDD prompt budget/inclusion config types, defaults, YAML tags.
- `internal/config/config_test.go` - defaults/override coverage for new fields.
- `internal/prompt/prompt.go` - add ATDD shaping API, deterministic trim order, ATDD rules subset extraction, truncation helper.
- `internal/prompt/prompt_test.go` (and/or new focused test file) - include/exclude, trim ordering, budget enforcement, bead identity retention.
- `internal/runner/callbacks.go` - shape context in ATDD render callback and emit stats logs.
- `internal/runner/methodology/executor.go` - ensure per-invocation metric log contract is preserved.
- `internal/runner/methodology/methodology_test.go` and/or `internal/runner/callbacks_test.go` - verify stats log presence and callback behavior.
- `.gromit/templates/PROMPT_acceptance_tests.md` - minimal adjustments for clear truncation markers if needed.
- `gromit.yaml` - document/comment new ATDD prompt settings.

**Files to Create:**
- `internal/prompt/atdd_prompt_budget_test.go` (recommended) - table-driven tests for shaping and trim priority.
- `internal/config/atdd_prompt_config_test.go` (optional, if keeping config tests isolated).

**Tradeoffs:**
- Chose **ATDD-only shaping at render time** over changing `BuildContext` globally to avoid regressions in build/review prompts.
- Chose **deterministic mechanical trimming** over LLM summarization for predictability and testability.
- Chose **rules subset fallback** over immediate rules drop to retain critical testing/process constraints under budget pressure.
- Chose **head+tail spec truncation** over head-only truncation to preserve both overview and concrete acceptance/details near the end.

## Test Strategy

**Test Levels:**
1. **Unit Tests**
- `internal/prompt`: validate ATDD context shaping behavior (toggles, caps, deterministic trim order, budget enforcement, mandatory bead identity retention, truncation marker behavior).
- `internal/config`: validate new ATDD prompt defaults and YAML overrides.
- `internal/runner`/`methodology`: validate per-attempt ATDD prompt observability logs and that shaped context is used for acceptance rendering.

2. **Integration Tests (targeted)**
- Runner/methodology wiring test where ATDD is active and acceptance render path logs `prompt_chars_before`, `prompt_chars_after`, section sizes, and `trim_actions`.
- Large-context fixture-style test in `internal/prompt` proving default ATDD budget yields >=30% prompt size reduction while preserving bead ID/title/description.

3. **Manual Verification**
- Run targeted test packages (`go test ./internal/prompt ./internal/config ./internal/runner/...` as needed).
- Optionally run a dry-run bead to inspect normal logs for ATDD prompt metrics and trim steps.

**Key Test Cases:**
- Include toggles:
  - `include_rules=false` excludes rules.
  - `include_spec=false` excludes spec.
  - `include_claude_md=false` excludes CLAUDE.
- Learnings cap:
  - `max_confirmed_learning_chars` enforces cap deterministically.
  - zero/unset behavior remains backward compatible where intended.
- Budget trim order:
  - On overflow, actions occur in exact order: drop recent learnings -> shrink/drop confirmed learnings -> rules subset -> spec truncate head/tail with marker -> drop rules entirely.
- Mandatory identity:
  - bead ID/title/description always survive shaping.
- Budget enforcement:
  - final rendered acceptance prompt length <= `max_chars` when possible with defined fallback.
- Observability:
  - one log event per ATDD acceptance invocation includes before/after char counts, section sizes, and trim action list.
- Compatibility:
  - behavior from `gromit-kse3` (CLAUDE exclusion) remains valid under defaults.

**Mocking Strategy:**
- Mock/stub render/invoke callbacks in methodology/runner tests (existing pattern).
- Use real prompt template rendering in prompt tests where needed to verify actual char counts and markers.
- Avoid provider/network dependencies; keep tests local and deterministic.

**Coverage Goals:**
- Critical path: ATDD acceptance render path from `bc.PromptCtx` -> shaped context -> prompt text -> logs.
- Edge cases:
  - tiny budgets,
  - empty sections,
  - no spec label,
  - no rules file,
  - `max_chars=0` (unlimited),
  - exact-boundary char counts.

**Test Organization:**
- Keep shaping behavior tests concentrated in `internal/prompt` with table-driven cases.
- Keep wiring/log tests in `internal/runner` or `internal/runner/methodology` near existing ATDD tests.
- Naming pattern: `TestATDDPromptShape_*`, `TestATDDPromptBudget_*`, `TestRunAcceptanceTests_LogsPromptStats`.

## Implementation Tasks

### Task 1: Add ATDD Prompt Budget Config Surface

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add ATDD prompt-specific config fields under `methodology` for inclusion toggles and budgets:
- `max_chars`
- `include_rules`
- `include_spec`
- `include_claude_md`
- `max_confirmed_learning_chars`
Set defaults to match the spec intent (CLAUDE disabled by default for ATDD, small confirmed learnings cap, and non-zero prompt budget unless explicitly configured otherwise). Ensure YAML unmarshalling and defaults are backward compatible.

**Acceptance Criteria:**
- Config loads without errors when new fields are omitted.
- Default values for ATDD prompt config are applied by `SetDefaults`.
- YAML overrides correctly change each ATDD prompt field.

**Dependencies:**
- None

**Notes:**
- Keep this scoped to ATDD; do not alter build/review config behavior.

### Task 2: Implement ATDD Context Shaper With Deterministic Trim Order

**Files:**
- Modify: `internal/prompt/prompt.go`
- Create: `internal/prompt/atdd_prompt_budget_test.go`

**What to Do:**
Add a shaping function that takes a full `*prompt.Context` plus ATDD prompt config and returns:
- shaped context (copy, not in-place mutation),
- before/after char counts,
- section-size stats,
- ordered `trim_actions`.
Apply toggles and confirmed learning char caps first, then enforce budget with deterministic priority:
1) drop recent learnings,
2) reduce confirmed learnings to cap then drop,
3) replace full rules with ATDD rules subset,
4) truncate spec with explicit marker while retaining head/tail,
5) drop rules entirely as last resort.
Never remove bead identity fields (`ID`, `Title`, `Description`).

**Acceptance Criteria:**
- Shaping is deterministic for identical inputs.
- Trim actions occur in required order under budget pressure.
- Bead identity fields are retained in the shaped context regardless of trim level.

**Dependencies:**
- Task 1

**Notes:**
- Keep trimming mechanical and local; no model calls.

### Task 3: Add ATDD Rules Subset Extraction

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/prompt/prompt_test.go`

**What to Do:**
Implement an ATDD-oriented rules subset helper that extracts only acceptance-writing-relevant constraints (test quality, no implementation edits in acceptance phase, expected-failure discipline). Integrate it into the shaper fallback path and ensure it can be selected deterministically when full rules exceed budget.

**Acceptance Criteria:**
- Rules subset generation is stable and excludes unrelated general build-rule content.
- Shaper uses subset before full rules omission.
- Existing non-ATDD `LoadRules` and `LoadRulesForPhase` behavior remains unchanged.

**Dependencies:**
- Task 2

**Notes:**
- Prefer deriving from existing `RULES.md` content rather than hardcoding large text blocks.

### Task 4: Wire ATDD Shaping Into Acceptance Rendering and Log Stats

**Files:**
- Modify: `internal/runner/callbacks.go`
- Modify: `internal/runner/methodology/executor.go`
- Modify: `internal/runner/methodology/methodology_test.go`
- Modify: `internal/runner/callbacks_test.go`

**What to Do:**
Update ATDD render callback wiring so acceptance prompts are rendered from shaped context using new ATDD config. Emit per-attempt observability logs in normal output including:
- `prompt_chars_before`
- `prompt_chars_after`
- `trim_actions`
- key section sizes (`rules/spec/claude/learnings`)
Ensure this happens once per acceptance invocation attempt and does not require debug mode.

**Acceptance Criteria:**
- ATDD acceptance path uses shaped context before `RenderAcceptanceTests`.
- Logs include required sizing and trim metadata once per acceptance invocation.
- Existing ATDD invocation/retry behavior remains intact.

**Dependencies:**
- Task 1
- Task 2
- Task 3

**Notes:**
- Keep logging format machine-scannable and stable for future regression checks.

### Task 5: Template Alignment and Compatibility Guardrails

**Files:**
- Modify: `.gromit/templates/PROMPT_acceptance_tests.md`
- Modify: `internal/prompt/prompt_test.go`

**What to Do:**
Adjust acceptance template wording only as needed for shaped context behavior (e.g., explicit truncation markers, optional sections disappearing under budget). Preserve existing semantics from earlier CLAUDE removal work and avoid broad prompt rewrite.

**Acceptance Criteria:**
- Template renders cleanly when rules/spec/CLAUDE/learnings are partially or fully omitted.
- Truncated spec markers are clear in rendered output when applied.
- Existing acceptance-template behavior remains compatible with prior CLAUDE removal expectations.

**Dependencies:**
- Task 2
- Task 3

**Notes:**
- Keep template edits minimal and ATDD-specific.

### Task 6: Add Large-Context Budget Regression Coverage

**Files:**
- Modify: `internal/prompt/atdd_prompt_budget_test.go`
- Modify: `internal/runner/methodology/methodology_test.go` (or existing ATDD wiring test file)

**What to Do:**
Add a large-context fixture-style test that simulates high rules/spec/learnings payload and verifies default ATDD budgeting yields at least 30% prompt-size reduction while preserving required bead/task identity. Include end-to-end acceptance render path coverage for stats emission.

**Acceptance Criteria:**
- Test demonstrates >=30% prompt reduction under default ATDD budget settings on large context.
- Test asserts required bead identity fields still present after shaping.
- Tests fail if trim order or observability metadata regresses.

**Dependencies:**
- Task 4
- Task 5

**Notes:**
- Keep fixture deterministic and local; avoid external files unless already present and stable.

---

## Notes

- This plan intentionally avoids changing non-ATDD prompt rendering paths.
- Budget enforcement should be deterministic and explainable from logs; avoid opaque trimming heuristics.
- If a strict budget cannot be met after all defined trim actions, retain mandatory bead identity and emit explicit trim metadata showing terminal state.
