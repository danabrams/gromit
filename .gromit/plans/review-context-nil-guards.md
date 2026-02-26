---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T18:48:06Z"
id: review-context-nil-guards
source_spec: review-context-nil-guards
---

# ReviewContext and ThoroughReviewContext Nil Guards Implementation Plan

**Goal:** Ensure review and TDD prompt contexts normalize nil slices/maps before rendering to prevent template panics or unexpected output.

**Architecture:** Add `normalizeNilFields()` helpers to the relevant context types and invoke them at render entry points prior to budget shaping and template execution.

**Tech Stack:** Go (internal prompt rendering and templates)

**Spec:** `.gromit/specs/review-context-nil-guards.md`

---

## Architecture

**Overview:**
Add per-context `normalizeNilFields()` helpers for review and TDD contexts, and invoke them at render entry points so templates always see non-nil slices/maps.

**Key Components:**
1. **`ReviewContext.normalizeNilFields()`**: Ensures `ValidationCommands` is `[]string{}` when nil.
2. **`ThoroughReviewContext.normalizeNilFields()`**: Ensures `CompletedBeads` is `[]CompletedBeadSummary{}` when nil.
3. **`TDDRedContext.normalizeNilFields()`**: Ensures `TestFileContents` is `map[string]string{}` when nil.
4. **`TDDGreenContext.normalizeNilFields()`**: Ensures `ImplFileContents` is `map[string]string{}` when nil.

**Integration Points:**
- Call `normalizeNilFields()` at the start of render functions:
  - `RenderReview`
  - `RenderThoroughReview`
  - `RenderTDDRed`
  - `RenderTDDGreen`
- This mirrors existing `Context` and `ScopeEstimate` normalization patterns.

**Data Flow:**
Caller constructs context -> `Render*` entrypoint normalizes nils -> optional budget shaping (review/thorough) -> template render executes with non-nil slices/maps.

**Files to Modify:**
- `internal/prompt/context_types.go` - add `normalizeNilFields()` for Review/Thorough/TDD contexts
- `internal/prompt/render_methods.go` - invoke normalization at render entry points
- `internal/prompt/context_types_test.go` - add nil-guard tests

**Files to Create:**
- None.

**Tradeoffs:**
- Normalizing at render entry avoids surprising nils without changing upstream callers or budget shapers.
- Keeps behavior consistent with existing `Context` and `ScopeEstimate` patterns.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Validate `normalizeNilFields()` on the new context types.
2. **Integration Tests**: Ensure render calls do not panic with nil slices/maps (if needed, add lightweight render tests).

**Key Test Cases:**
- `ReviewContext` with `ValidationCommands == nil` normalizes to non-nil empty slice.
- `ReviewContext` with non-nil `ValidationCommands` preserves contents.
- `ThoroughReviewContext` with `CompletedBeads == nil` normalizes to non-nil empty slice.
- `TDDRedContext` with `TestFileContents == nil` normalizes to non-nil empty map.
- `TDDGreenContext` with `ImplFileContents == nil` normalizes to non-nil empty map.
- Render entrypoints accept contexts with nil slices/maps without panic and execute (optional, minimal template setup).

**Mocking Strategy:**
- No mocks needed; use minimal contexts and templates where render is exercised.

**Coverage Goals:**
- Nil and empty inputs are treated equivalently.
- Render functions are safe under nil fields.

**Test Organization:**
- Add new tests to `internal/prompt/context_types_test.go`.
- Add render safety tests to `internal/prompt/prompt_test.go` only if needed.

## Implementation Tasks

### Task 1: Add nil normalization helpers for review and TDD contexts

**Files:**
- Modify: `internal/prompt/context_types.go`

**What to Do:**
- Add `normalizeNilFields()` methods on `ReviewContext`, `ThoroughReviewContext`, `TDDRedContext`, and `TDDGreenContext`.
- Follow existing `Context`/`ScopeEstimate` conventions (nil receiver safe, unexported helper).

**Acceptance Criteria:**
- `ReviewContext.normalizeNilFields()` converts nil `ValidationCommands` to `[]string{}`.
- `ThoroughReviewContext.normalizeNilFields()` converts nil `CompletedBeads` to `[]CompletedBeadSummary{}`.
- `TDDRedContext.normalizeNilFields()` initializes nil `TestFileContents` to empty map.
- `TDDGreenContext.normalizeNilFields()` initializes nil `ImplFileContents` to empty map.

**Dependencies:**
- None.

**Notes:**
- Keep helper methods unexported and nil-receiver safe.

### Task 2: Normalize contexts at render entry points

**Files:**
- Modify: `internal/prompt/render_methods.go`

**What to Do:**
- Call `ctx.normalizeNilFields()` at the start of `RenderReview`, `RenderThoroughReview`, `RenderTDDRed`, and `RenderTDDGreen`.
- Ensure normalization occurs before budget shaping or diagnostics calculation.

**Acceptance Criteria:**
- All listed render methods normalize their contexts before use.
- Existing behavior is preserved aside from nil normalization.

**Dependencies:**
- Task 1.

**Notes:**
- Follow the pattern used by build context normalization in `BuildContext`.

### Task 3: Add nil-guard tests

**Files:**
- Modify: `internal/prompt/context_types_test.go`
- Modify: `internal/prompt/prompt_test.go` (optional, if render safety tests are added)

**What to Do:**
- Add table-driven unit tests covering nil and non-nil cases for each new `normalizeNilFields()` helper.
- Optionally add lightweight render tests to confirm no panic on nil slices/maps.

**Acceptance Criteria:**
- Tests cover nil inputs and verify non-nil empty slices/maps after normalization.
- Tests confirm existing values are preserved when non-nil.
- Test suite passes.

**Dependencies:**
- Task 1.

**Notes:**
- Prefer unit tests in `context_types_test.go`; add render tests only if necessary for confidence.

---

## Notes

- Keep normalization consistent with existing patterns in `Context` and `ScopeEstimate`.
- Ensure render entry points normalize before budget shaping to avoid nil-sensitive trim logic.
