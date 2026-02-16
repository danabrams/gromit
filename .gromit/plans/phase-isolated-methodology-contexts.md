---
created: 2026-02-16T00:00:00Z
decomposed: true
decomposed_at: "2026-02-16T18:12:02Z"
id: phase-isolated-methodology-contexts
source_spec: phase-isolated-methodology-contexts
---

# Phase-Isolated Contexts for Red/Green/Refactor/Validation Implementation Plan

**Goal:** Execute methodology phases in isolated sibling contexts with phase-specific timeout budgets and phase-attributed failure semantics, while preserving backward compatibility.

**Architecture:** Add optional phase-timeout config and a shared phase-context builder that derives fresh contexts from `ParentCtx` per phase; wire methodology orchestration and validation to use these contexts; add phase-scoped prompt shaping for red/green/refactor.

**Tech Stack:** Go, existing runner/methodology/validation/prompt packages, YAML config parsing/defaulting, Go test.

**Spec:** `.gromit/specs/phase-isolated-methodology-contexts.md`

---

## Architecture

## Architecture Proposal

**Overview:**  
Introduce explicit sibling phase contexts (`red`, `green`, `refactor`, `validation`) derived from `ParentCtx`, with phase-specific timeout resolution and phase-aware error classification/logging. Keep existing semantics by default when phase timeout config is unset.

**Key Components:**
1. **Phase timeout config model (`internal/config/config.go`)**: Add optional methodology/validation phase timeout fields and default/fallback resolvers.
2. **Phase context builder (`internal/runner`, new helper)**: Create a small helper that derives a per-phase `context.WithTimeout` from `bc.ParentCtx`, clamped by remaining run budget/deadline.
3. **Methodology orchestration updates (`internal/runner/process_methodology.go`)**: Execute red/green/refactor/validation using explicit phase contexts instead of reusing bead context.
4. **Phase-aware error wrapping/classification (`internal/runner/process_methodology.go`, `internal/runner/runtypes/types.go`, formatting/logging files as needed)**: Attribute timeout/cancel errors to the exact phase.
5. **Minimal phase prompt shaping (`internal/prompt/prompt.go`, runner callbacks/methodology wiring)**: Add phase-scoped context shaping and use it for acceptance/build/refactor prompts.

**Integration Points:**
- `setupBeadContext` still owns run/bead envelopes; phase helpers consume `bc.ParentCtx` and `bc.RunDeadline`.
- `runATDDPreBuildPhases`, build invocation path, and `runRefactorAndPostChecks` become phase-context-driven.
- Existing `validation.Runner` remains command authority; orchestration supplies validation-phase context.
- Existing renderer/template registration pattern remains; shaping is additive.

**Data Flow:**
- For methodology-enabled bead:
  1. Build `redCtx` from `ParentCtx` + red timeout; run acceptance generation and verify-fail.
  2. Build `greenCtx`; run build implementation invocation.
  3. Build `refactorCtx`; run optional refactor.
  4. Build `validationCtx`; run direct validation + recovery loop.
- Each phase reports timeout/cancel with phase label; optional refactor failures remain non-fatal unless policy says otherwise.

**Files to Modify:**
- `internal/config/config.go` - add phase-timeout structs/fields, defaults, fallback resolvers.
- `internal/config/config_test.go` - defaulting + YAML override + backward-compat tests.
- `internal/runner/process_methodology.go` - construct/use phase contexts and phase-specific wrapping.
- `internal/runner/process.go` - ensure green/build phase context is used for methodology-enabled build path.
- `internal/runner/runtypes/types.go` - additive phase-attribution fields if needed for iteration output.
- `internal/runner/format.go` and/or `internal/runner/logging.go` - render phase timeout/cancel info.
- `internal/prompt/prompt.go` - phase-scoped shaping helpers + minimal render path support.
- `internal/runner/callbacks.go` - wire methodology render callbacks through phase shaper/rules-per-phase.

**Files to Create:**
- `internal/runner/phase_context.go` - timeout resolution + `newPhaseContext(...)` helper.
- `internal/prompt/methodology_phase_context.go` - shaping functions for red/green/refactor.
- `internal/prompt/methodology_phase_context_test.go` - shaping behavior tests.

**Tradeoffs:**
- **Central phase context helper vs inline context logic:** chose helper for consistent timeout clamping and attribution across phases.
- **Additive config fields vs replacing existing timeout fields:** chose additive for backward compatibility and low migration risk.
- **Prompt shaping in renderer vs runner:** chose renderer-side shaping for template compatibility and testability near prompt code.

---

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Verify timeout resolution/default fallbacks, phase-context creation semantics, phase-specific error wrapping/classification, and prompt shaping behavior.
2. **Integration-Style Runner Tests**: Exercise methodology orchestration across red/green/refactor/validation with controlled fake callbacks to verify context isolation and phase attribution.
3. **Manual/Smoke Verification**: Run targeted runner/prompt/config test packages and one representative `gromit run -n 1` in a controlled scenario (if available) to confirm no regressions in default behavior.

**Key Test Cases:**
- Refactor phase context times out, but validation still executes successfully via a fresh validation phase context from `ParentCtx`.
- Validation timeout is classified as `validation` phase timeout, distinct from refactor timeout.
- When phase timeout fields are unset/zero, fallback uses existing defaults (`claude.bead_timeout` for model-driven phases, existing validation behavior).
- Required phase failure (green/build) remains terminal; optional phase failure (refactor timeout) remains non-fatal unless already-required follow-up fails.
- Validation phase timeout caps full recovery loop while `command_timeout` continues to cap individual commands.
- Prompt shaping for red/green/refactor includes required core sections and trims low-signal sections deterministically.
- Existing non-methodology beads still run with unchanged behavior.

**Mocking Strategy:**
- Mock/fake methodology callbacks (`renderFn`, `invokeFn`, `validateFn`) to deterministically trigger deadline/cancel/success paths.
- Mock command runner for validation timeout and retry timing scenarios.
- Keep real config load/defaulting behavior in config tests (YAML parse + `SetDefaults`/`NormalizeNilFields`).

**Coverage Goals:**
- Critical paths:
  - Phase context derivation from `ParentCtx`
  - Timeout budget fallback and clamping
  - Phase-aware classification/log output
  - Validation recovery loop under phase timeout cap
- Edge cases:
  - Run deadline nearly exhausted before a phase starts
  - Zero/unset timeout config fields
  - Parent context canceled before phase start
  - Optional phase timeout without forcing bead failure

**Test Organization:**
- `internal/config/config_test.go`: phase-timeout config/defaulting tests.
- `internal/runner/process_methodology_context_test.go` (expand): orchestration context isolation scenarios.
- `internal/runner/refactor_validation_error_test.go` (expand or sibling): phase-specific timeout message/classification checks.
- `internal/prompt/*_test.go` (new+existing): phase prompt shaping/minimal context tests.
- Any new helper tests colocated with helpers (`internal/runner/phase_context_test.go`, etc.).

---

## Implementation Tasks

### Task 1: Add Phase Timeout Configuration Surface

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add optional phase-timeout fields for methodology and validation phases, with defaulting and fallback logic that preserves current behavior when unset. Include helper methods to resolve effective timeouts per phase.

**Acceptance Criteria:**
- `methodology.phase_timeouts.red_seconds|green_seconds|refactor_seconds` and `validation.phase_timeout_seconds` are parsed from YAML.
- `SetDefaults`/resolver methods preserve existing behavior when new fields are zero/unset.
- Config tests cover defaults, overrides, and backward compatibility.

**Dependencies:**
- None

**Notes:**
- Keep additive-only schema changes; do not break existing config keys.

### Task 2: Implement Shared Phase Context Builder

**Files:**
- Create: `internal/runner/phase_context.go`
- Create: `internal/runner/phase_context_test.go`

**What to Do:**
Introduce helper(s) that create a fresh phase context from `bc.ParentCtx` with phase timeout resolution and run-deadline clamping, returning `(ctx, cancel, metadata)` for logging/classification.

**Acceptance Criteria:**
- Phase contexts are always derived from `ParentCtx` and never chained from prior phase contexts.
- Effective timeout is clamped by remaining run budget and handles near-expiry safely.
- Tests cover unset timeout fallback, explicit timeout override, parent-canceled behavior, and run-deadline clamp.

**Dependencies:**
- Task 1 (timeout resolvers)

**Notes:**
- Keep helper generic for reuse by red/green/refactor/validation orchestration.

### Task 3: Wire Red/Green/Refactor/Validation to Isolated Contexts

**Files:**
- Modify: `internal/runner/process_methodology.go`
- Modify: `internal/runner/process.go`
- Modify: `internal/runner/methodology/executor.go`
- Modify: `internal/runner/methodology/refactor.go`
- Modify: `internal/runner/process_methodology_context_test.go`

**What to Do:**
Refactor methodology orchestration to create explicit phase contexts for red, green, refactor, and validation. Ensure validation after refactor always uses a fresh validation phase context even if refactor timed out/canceled.

**Acceptance Criteria:**
- Red, green, refactor, and validation execute with separate sibling contexts from `ParentCtx`.
- Refactor timeout/cancel does not pre-cancel validation phase context.
- Required/optional phase semantics remain intact (green required, refactor optional by current policy).

**Dependencies:**
- Task 2

**Notes:**
- Keep non-methodology flow unchanged.

### Task 4: Add Phase-Aware Timeout/Cancellation Classification and Logging

**Files:**
- Modify: `internal/runner/runtypes/types.go`
- Modify: `internal/runner/process_methodology.go`
- Modify: `internal/runner/format.go`
- Modify: `internal/runner/logging.go`
- Modify: `internal/runner/refactor_validation_error_test.go`
- Modify: `internal/runner/format_test.go`

**What to Do:**
Propagate phase name (`red|green|refactor|validation`) into timeout/cancel error wrapping and iteration output so failures are attributed to exact phase rather than generic bead exhaustion.

**Acceptance Criteria:**
- Timeout/cancel errors include explicit phase attribution in error text and/or result fields.
- Validation timeout after refactor is classified as validation-phase timeout.
- Output/format tests assert new phase attribution fields/messages.

**Dependencies:**
- Task 3

**Notes:**
- Keep schema additive to avoid breaking existing consumers.

### Task 5: Implement Minimal Phase Prompt Shaping in Renderer

**Files:**
- Create: `internal/prompt/methodology_phase_context.go`
- Create: `internal/prompt/methodology_phase_context_test.go`
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/prompt/prompt_test.go`
- Modify: `internal/runner/callbacks.go`
- Modify: `internal/runner/methodology/methodology_test.go`

**What to Do:**
Add phase-scoped prompt-context shaping helpers for red/green/refactor that trim low-signal sections by default while preserving required constraints (`RULES.md`, acceptance criteria/spec context, failure signal, behavior-preservation guardrails).

**Acceptance Criteria:**
- Red prompt path includes acceptance/spec context and test-only guardrails with minimal extraneous context.
- Green prompt path emphasizes failing-test signal and required behavior without full-history duplication.
- Refactor prompt path uses diff/constraints focus and excludes redundant build payload.
- Renderer/callback tests verify shaped context is used and template rendering remains compatible.

**Dependencies:**
- None (can start in parallel with Tasks 1-3; final wiring depends on Task 3)

**Notes:**
- Reuse existing templates where possible; only minimal wording updates if strictly needed.

### Task 6: Validation Timeout Budget Enforcement for Full Validation Phase

**Files:**
- Modify: `internal/runner/validation/runner.go`
- Modify: `internal/runner/validation/validation_test.go`
- Modify: `internal/runner/process_methodology_context_test.go`

**What to Do:**
Ensure validation-phase context budget caps full validation flow (including recovery loop), while preserving per-command `validation.command_timeout` behavior.

**Acceptance Criteria:**
- Recovery loop respects validation phase context deadline and exits with phase timeout classification.
- Per-command timeouts still trigger command-level timeout handling as before.
- Tests cover interaction between phase timeout cap and command timeout.

**Dependencies:**
- Task 1
- Task 2
- Task 3

**Notes:**
- Keep `validation.Runner` as execution authority; orchestration should pass the right phase context.

### Task 7: End-to-End Reliability Regression Coverage

**Files:**
- Modify: `internal/runner/process_methodology_context_test.go`
- Modify: `internal/runner/methodology/methodology_test.go`
- Modify: `internal/config/config_test.go`

**What to Do:**
Add or refine integration-style tests directly matching acceptance criteria from the spec and decisions, including timeout isolation and fallback behavior.

**Acceptance Criteria:**
- Test proves refactor timeout followed by successful validation in separate context.
- Test proves validation timeout classification independent of refactor timeout.
- Test proves default fallback behavior when phase timeout config is omitted.

**Dependencies:**
- Tasks 1-6

**Notes:**
- Keep tests deterministic using fake callbacks/short in-memory timing controls.

### Task 8: Verification Sweep and Compatibility Check

**Files:**
- Modify (if needed): any touched files from Tasks 1-7

**What to Do:**
Run focused and full quality gates for changed packages, fix regressions, and confirm no behavior change in non-methodology paths.

**Acceptance Criteria:**
- Targeted tests for `internal/config`, `internal/prompt`, `internal/runner`, and `internal/runner/validation` pass.
- `go test ./...` (or agreed lane) passes for this branch.
- No regressions detected in non-methodology flows from existing tests.

**Dependencies:**
- Tasks 1-7

**Notes:**
- Keep changes minimal and scoped to spec intent.

---

## Notes

- This plan intentionally keeps provider routing/escalation policy unchanged and focuses only on context isolation, timeout budgets, prompt minimization, and phase-level diagnostics.
- When implementing, prefer additive fields and helpers to minimize breakage risk and preserve current defaults.
- Decomposition should keep beads focused (1-2 tightly-coupled files per bead where practical) while preserving dependency order above.
