---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T19:12:58Z"
id: three-way-cross-review-routing
source_spec: three-way-cross-review-routing
---

# Three-Way Cross-Review Routing Implementation Plan

**Goal:** Ensure `review: cross` always prefers any available non-build provider in two- and three-provider setups (including Gemini), with fallback to the build provider when no alternative is available.

**Architecture:** Reuse existing `SelectCross` and review-phase routing gate, and harden acceptance coverage with explicit three-provider selection and fallback tests while preserving all non-cross behavior.

**Tech Stack:** Go, existing provider router/reviewer abstractions, table-driven unit tests (`testing` package).

**Spec:** `.gromit/specs/three-way-cross-review-routing.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Keep routing logic in `Router.SelectCross` unchanged, and tighten acceptance-level coverage for explicit three-provider cross-review permutations (including Gemini) plus fallback semantics when alternatives are unavailable.

**Key Components:**
1. **`internal/provider/router.go`**: keep as single source of truth for cross-review selection and fallback.
2. **`internal/runner/reviewpkg/reviewer.go`**: keep cross-routing gate at review phase only.
3. **`internal/provider/cross_review_test.go`**: expand tests to directly encode this spec’s acceptance criteria.

**Integration Points:**
- No new runtime integration points.
- Behavior remains scoped to `review: cross`.
- Non-cross routes (`review: claude|openai|any|...`) remain untouched.

**Data Flow:**
- Build phase records `buildProvider`.
- Review phase checks preference:
  - if `cross` and build provider known: call `SelectCross(buildProvider, tier)`
  - else: call `Select(review, tier)`
- `SelectCross` chooses non-build available provider if possible; otherwise build provider; otherwise none.

**Files to Modify:**
- `internal/provider/cross_review_test.go` - add explicit 3-provider matrix + unavailable-alternative fallback coverage.
- `internal/runner/reviewpkg/reviewer_test.go` - add/adjust guard tests to confirm non-cross routing path remains unchanged.

**Files to Create:**
- None expected.

**Tradeoffs:**
- Chose **test-focused refinement** over router refactor because core behavior already matches spec.
- Kept **non-deterministic alternative selection** (map iteration) because spec allows any non-build provider.
- Avoided changing selection strategy to preserve existing routing behavior and minimize regression risk.

## Test Strategy

**Test Levels:**
1. **Unit Tests (`internal/provider`)**: verify `SelectCross` semantics directly across three-provider combinations and availability states.
2. **Unit Tests (`internal/runner/reviewpkg`)**: verify review routing gate still only uses cross logic when configured; non-cross behavior remains unchanged.
3. **Manual Testing**: optional config smoke (`review: cross`) is low value here since behavior is deterministic and already isolated by unit tests.

**Key Test Cases:**
- `review: cross`, providers `claude/openai/gemini`, build=`gemini` -> selected provider is `claude` or `openai`, never `gemini` when alternatives are available.
- `review: cross`, providers `claude/openai/gemini`, build=`claude` -> selected provider is `openai` or `gemini`, never `claude` when alternatives are available.
- `review: cross`, all non-build providers unavailable, build provider available -> returns build provider.
- single configured provider + `review: cross` -> returns that provider (fallback behavior).
- no provider available -> returns nil provider (terminal failure behavior).
- non-cross review preferences continue using `Select`, not `SelectCross`.

**Mocking Strategy:**
- Use existing `mockProviderWithModels` and router fixture style in `cross_review_test.go`.
- Use `mockRouter` in `reviewer_test.go` to assert call-path behavior (`Select` vs `SelectCross`) without real provider execution.

**Coverage Goals:**
- Fully cover the explicit acceptance criteria for three-provider cross-review + fallback.
- Protect against regression where cross mode could incorrectly select build provider while alternatives exist.
- Protect unchanged non-cross routing paths.

**Test Organization:**
- Extend `internal/provider/cross_review_test.go` with table-driven subtests for build provider permutations and unavailable alternatives.
- Add focused regression test(s) in `internal/runner/reviewpkg/reviewer_test.go` for non-cross route invariants.
- Keep names aligned with behavior, e.g. `TestSelectCrossWithThreeProviders_*` and `TestRunLight_UsesStandardRoutingWhenReviewNotCross`.

## Implementation Tasks

### Task 1: Add Explicit Three-Provider Cross-Selection Acceptance Tests

**Files:**
- Modify: `internal/provider/cross_review_test.go`

**What to Do:**
Add table-driven tests that explicitly cover three-provider cross-review permutations required by the spec, including beads built by `gemini` and `claude`. Assert that when any alternative providers are available, selected provider is never the build provider and is one of the valid alternatives.

**Acceptance Criteria:**
- `buildProvider=gemini` with available `claude/openai` selects either `claude` or `openai`, never `gemini`.
- `buildProvider=claude` with available `openai/gemini` selects either `openai` or `gemini`, never `claude`.
- Tests remain resilient to non-deterministic map iteration by asserting membership, not exact provider.

**Dependencies:**
- None.

**Notes:**
Use helper assertions for allowed-provider sets to keep subtests concise and avoid flaky exact-match expectations.

### Task 2: Add Three-Provider Unavailable-Alternative Fallback Test Coverage

**Files:**
- Modify: `internal/provider/cross_review_test.go`

**What to Do:**
Add/expand fallback tests for three-provider setups where both non-build providers are unavailable while the build provider is available, asserting `SelectCross` falls back to build provider. Include no-provider-available path coverage if currently absent in this file.

**Acceptance Criteria:**
- When both non-build providers are unavailable and build provider is available, selection returns build provider.
- When all providers are unavailable, selection returns `nil` provider and empty model.
- Existing single-provider fallback test continues to pass unchanged.

**Dependencies:**
- Task 1 (same fixture patterns can be reused).

**Notes:**
Use `unavailable` timestamps in the future to force unavailable status through existing router checks.

### Task 3: Preserve Non-Cross Review Routing Behavior with Reviewer Guard Tests

**Files:**
- Modify: `internal/runner/reviewpkg/reviewer_test.go`

**What to Do:**
Add a reviewer regression test ensuring non-cross review preferences (`claude`, `openai`, `any`) continue using `Select` and do not call `SelectCross`, even when `buildProvider` is present. Keep existing cross-review gate tests intact.

**Acceptance Criteria:**
- For non-cross review preference, `RunLight` invokes `Select` and not `SelectCross`.
- Existing cross-review test still verifies `SelectCross` is used only when review preference is `cross` and build provider is set.
- No behavior change to prompt rendering or review result parsing paths.

**Dependencies:**
- None (can run independently of router tests).

**Notes:**
Use boolean call flags on `mockRouter` methods to keep assertions precise and isolated.

### Task 4: Run Targeted Quality Gates for Routing and Review Tests

**Files:**
- Test only (no source file changes expected).

**What to Do:**
Run focused test commands for provider cross-review and review package routing to confirm no regressions and that new acceptance coverage passes reliably.

**Acceptance Criteria:**
- `go test ./internal/provider -run SelectCross` passes.
- `go test ./internal/runner/reviewpkg -run RunLight` (or equivalent focused pattern) passes.
- No unrelated test failures are introduced by these changes.

**Dependencies:**
- Task 1
- Task 2
- Task 3

**Notes:**
If test names differ after implementation, use equivalent regex patterns that execute the new and existing related tests.

---

## Notes

- Runtime routing code already appears aligned with this spec; this plan is intentionally biased toward acceptance-criteria hardening via tests.
- Keep non-cross routing paths untouched to minimize regression risk.
- During implementation, avoid introducing deterministic ordering constraints unless product requirements change; current spec allows any valid non-build provider choice.
