---
created: 2026-02-22T00:00:00Z
decomposed: true
decomposed_at: "2026-02-22T21:20:31Z"
id: decompose-low-complexity-bias
source_spec: decompose-low-complexity-bias
---

# Decompose Low Complexity Bias Implementation Plan

**Goal:** Bias `gromit decompose` output toward low-complexity, low-tier-executable beads using adaptive complexity-aware reprompting, without hard-blocking high-complexity fallback.

**Architecture:** Add a decompose-only complexity assessment (candidate fields + plan context), integrate targeted complexity reprompts into the existing retry loop, and emit per-attempt/final complexity summaries while still creating beads after retry limits.

**Tech Stack:** Go (`internal/pipeline`, `internal/validate`), existing decompose prompt/validation flow, Go test framework.

**Spec:** `.gromit/specs/decompose-low-complexity-bias.md`

---

## Architecture

**Overview:**  
Add a complexity-assessment and adaptive-reprompt loop inside the `gromit decompose` pipeline path, using multi-signal heuristics (candidate fields + plan context) to bias output toward low-complexity beads while still allowing high-complexity fallback with warnings.

**Key Components:**
1. **`internal/validate` complexity evaluator:** Scores each candidate bead using title/description/criteria/files/dependencies plus plan task context and emits classification + reasons.
2. **`internal/validate` reprompt feedback extensions:** Builds targeted reprompt feedback specifically for high-complexity beads (split concerns, reduce breadth, preserve semantics, avoid overlap).
3. **`internal/pipeline/decompose.go` adaptive retry coordinator:** Runs attempts, evaluates complexity outcomes each round, reprompts when high-complexity remains, stops on success/non-improvement/max retries, and proceeds with warnings when needed.
4. **Attempt/final reporting in decompose output:** Emits concise per-attempt complexity summary and final remaining high-complexity count + reasons.

**Integration Points:**
- Keep existing schema/overlap validation behavior intact and layer complexity assessment on top for decompose-only behavior.
- Reuse existing reprompt plumbing by adding complexity-specific feedback payloads.
- Keep runtime decomposition outside `gromit decompose` unchanged by scoping logic to plan decomposition orchestration.

**Data Flow:**
1. Decompose model returns candidate beads.
2. Existing validation runs (format/quality/overlap checks).
3. New complexity evaluator scores/classifies each candidate against candidate fields + plan task context.
4. If high-complexity exists, generate structured reprompt feedback and retry.
5. Retry loop tracks improvement trajectory:
   - stop when high-complexity count reaches zero
   - stop when no improvement from prior best
   - stop at retry ceiling
6. Final output creates beads regardless; if high-complexity remains, print warning summary with reasons.

**Files to Modify:**
- `internal/pipeline/decompose.go` - adaptive retry orchestration, attempt summaries, final warning behavior.
- `internal/validate/validate.go` - invoke complexity evaluation and expose outcomes for decompose loop.
- `internal/validate/reprompt.go` - generate targeted complexity-reduction feedback prompts.
- `internal/pipeline/decompose_test.go` - retry-stop, improvement, and warning-path tests.
- `internal/validate/validate_test.go` - complexity integration coverage.
- `internal/validate/reprompt_test.go` - targeted complexity feedback tests.

**Files to Create:**
- `internal/validate/complexity.go` - heuristic scorer/classifier and reason extraction.
- `internal/validate/complexity_test.go` - unit tests for multi-signal scoring and context-aware classification.

**Tradeoffs:**
- **Heuristic scoring vs strict threshold-only rules:** Heuristics better detect real-world overscope across mixed signals.
- **Decompose-local orchestration vs global behavior change:** Localized to preserve non-goal of runtime decomposition changes.
- **Retry-until-practical vs hard-blocking:** Bias-not-ban avoids delivery stalls when practical decomposition limits are reached.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** Validate complexity scoring/classification behavior in isolation (signals from title/description/criteria/files/dependencies + plan context).
2. **Integration Tests:** Exercise decompose adaptive retry orchestration end-to-end with mocked model outputs across attempts.
3. **Manual Testing:** Run `gromit decompose <plan>` on intentionally broad tasks to verify summaries/warnings and non-blocking behavior.

**Key Test Cases:**
- Context-aware high-complexity detection where candidate text alone appears simple.
- Mixed-concern detection for candidates combining independently deliverable behaviors.
- Retry success path: high-complexity on attempt 1, zero remaining before max retries.
- Non-improving stop path: retries stop when trajectory no longer improves.
- Retry-cap stop path: retries cap out and decomposition still proceeds.
- Warning fallback path: final output reports remaining high-complexity count and reasons.
- Regression guard: runtime decomposition paths outside `gromit decompose` unchanged.

**Mocking Strategy:**
- Mock provider responses by attempt to control improvement trajectories deterministically.
- Keep validation/complexity logic real in integration tests; mock only model IO and external boundaries.

**Coverage Goals:**
- Critical paths: multi-signal aggregation, stop conditions, warning-but-proceed behavior.
- Edge cases: empty fields, conflicting signals, dependency breadth indicators, no-change/tied attempts.

**Test Organization:**
- `internal/validate/complexity_test.go`: scoring/classification unit tests.
- `internal/validate/validate_test.go`: complexity integration coverage.
- `internal/validate/reprompt_test.go`: targeted feedback generation.
- `internal/pipeline/decompose_test.go`: loop orchestration and reporting behavior.

## Implementation Tasks

### Task 1: Add complexity assessment types and heuristic scorer

**Files:**
- Create: `internal/validate/complexity.go`
- Test: `internal/validate/complexity_test.go`

**What to Do:**
Implement complexity result types and a scorer that evaluates each candidate bead using candidate fields and plan context. Include explicit reason generation for high-complexity classification (broad scope language, mixed concerns, multi-package breadth, criteria breadth, dependency shape).

**Acceptance Criteria:**
- Scorer consumes both candidate data and plan context inputs.
- Classification is not based solely on estimated file count.
- High-complexity results include machine-usable and human-readable reasons.

**Dependencies:**
- None

**Notes:**
- Keep signal weighting configurable enough for later tuning without broad refactors.

### Task 2: Integrate complexity outcomes into validation pipeline

**Files:**
- Modify: `internal/validate/validate.go`
- Test: `internal/validate/validate_test.go`

**What to Do:**
Wire complexity scoring into decompose validation outputs so each attempt produces a complexity summary payload usable by pipeline orchestration and logging.

**Acceptance Criteria:**
- Validation result contains per-candidate complexity classification and reasons.
- Existing validation checks (criteria/scope/overlap) remain intact and behaviorally compatible.
- Validation integration tests cover mixed low/high candidate sets.

**Dependencies:**
- Task 1

**Notes:**
- Keep API additions minimal and backward compatible for existing callers where possible.

### Task 3: Extend reprompt feedback with complexity-reduction guidance

**Files:**
- Modify: `internal/validate/reprompt.go`
- Test: `internal/validate/reprompt_test.go`

**What to Do:**
Add structured feedback generation for high-complexity candidates, instructing finer decomposition while preserving semantic intent and preventing sibling overlap.

**Acceptance Criteria:**
- Reprompt feedback includes targeted reasons tied to flagged candidates.
- Guidance asks for decomposition refinement without changing original intent.
- Tests verify feedback formatting and inclusion rules for high-complexity-only cases.

**Dependencies:**
- Task 2

**Notes:**
- Reuse existing reprompt composition patterns to avoid prompt drift.

### Task 4: Implement adaptive retry loop improvements in decompose orchestration

**Files:**
- Modify: `internal/pipeline/decompose.go`
- Test: `internal/pipeline/decompose_test.go`

**What to Do:**
Update the decompose retry loop to evaluate complexity each attempt, reprompt while high-complexity remains, stop early at zero high-complexity, stop on non-improving trajectories, and enforce retry ceilings.

**Acceptance Criteria:**
- Loop continues while high-complexity remains and improvement is possible.
- Loop stops on any of: zero-high, non-improving attempts, retry cap reached.
- Decompose proceeds to bead creation even when high-complexity remains at loop exit.

**Dependencies:**
- Task 2
- Task 3

**Notes:**
- Improvement metric should be stable and deterministic for tests (e.g., high count then score tie-break).

### Task 5: Add attempt/final complexity reporting and warning summaries

**Files:**
- Modify: `internal/pipeline/decompose.go`
- Test: `internal/pipeline/decompose_test.go`

**What to Do:**
Emit concise per-attempt complexity summaries and final warning output when high-complexity beads remain after retries, including count and reason snippets.

**Acceptance Criteria:**
- Each attempt logs complexity outcome summary (including high-complexity count).
- Final output warns clearly when high-complexity remains and explains why.
- Reporting tests assert message presence and key fields.

**Dependencies:**
- Task 4

**Notes:**
- Keep output concise and actionable for users running `gromit decompose` interactively.

### Task 6: Regression and scope-boundary validation

**Files:**
- Modify: `internal/pipeline/decompose_test.go`
- Modify: `internal/validate/validate_test.go`

**What to Do:**
Add targeted regression tests proving the behavior applies to plan decomposition only and does not alter runtime decomposition behavior in other pipeline stages.

**Acceptance Criteria:**
- Tests demonstrate new complexity optimization is active in `gromit decompose`.
- Tests demonstrate non-plan/runtime decomposition behavior remains unchanged.
- Existing non-related decomposition tests stay green.

**Dependencies:**
- Task 4
- Task 5

**Notes:**
- Prefer focused regression tests over broad snapshot assertions.

---

## Notes

- This plan intentionally biases decomposition quality toward low-tier executability but does not hard-fail on remaining high-complexity output.
- Complexity heuristics are expected to evolve; prioritize readable signals and diagnostics to support iterative tuning.
- Keep implementation scoped to decompose orchestration and validation integration to satisfy non-goals.
