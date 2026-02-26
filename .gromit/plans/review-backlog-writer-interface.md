---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T18:47:03Z"
id: review-backlog-writer-interface
source_spec: review-backlog-writer-interface
---

# Review Backlog Writer Interface Implementation Plan

**Goal:** Narrow the review pipeline backlog dependency to a write-only interface so review adapters expose only methods the review workflow actually uses.

**Architecture:** Introduce a review-scoped `BacklogWriter` interface (`Add`, `Update`) in the pipeline layer, wire review code to that dependency, and remove dead `List`/`Get` stubs from `cliBacklogClient` while preserving non-review `BacklogClient` usage.

**Tech Stack:** Go, standard library, existing `internal/pipeline` and `cmd/gromit` packages

**Spec:** `.gromit/specs/review-backlog-writer-interface.md`

---

## Architecture

**Overview:**
Use an interface split at the pipeline dependency boundary: keep `BacklogClient` for workflows that require read methods (`List`, `Get`) and introduce `BacklogWriter` for review workflows that only create/update backlog entries.

**Key Components:**
1. **`pipeline.BacklogWriter`**: New interface with only `Add(item *Idea) error` and `Update(id string, fn func(*Idea)) error`.
2. **Review-specific dependency wiring in `pipeline.Deps`**: Add a dedicated review backlog writer field so review validation and execution can depend on the narrower contract.
3. **`cmd/gromit` review backlog adapter cleanup**: `cliBacklogClient` implements only writer methods and is compile-time asserted against `pipeline.BacklogWriter`.

**Integration Points:**
- `internal/pipeline/pipeline.go`:
  - Define `BacklogWriter` next to backlog interfaces.
  - Update review workflow dependency validation and usage to require writer dependency.
  - Preserve existing `BacklogClient` usage for refine/explore/status.
- `cmd/gromit/review.go`:
  - Wire `cliBacklogClient` into review writer dependency field.
  - Remove `List`/`Get` from `cliBacklogClient`.
  - Add compile-time check: `var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)`.

**Data Flow:**
1. `ReviewNonInteractive` parses review output.
2. For backlog findings, review workflow calls backlog writer `Add`.
3. Adapter creates backlog beads with review labels and expected outputs.
4. Review returns counts and persists log/state as before.

**Files to Modify:**
- `internal/pipeline/pipeline.go` - add `BacklogWriter`, update review dependency typing and validation.
- `internal/pipeline/review_test.go` - adjust review mocks/dependency setup and nil-dependency expectations.
- `cmd/gromit/review.go` - remove dead methods from `cliBacklogClient`, update compile-time assertion, and dependency wiring.
- `cmd/gromit/review_test.go` - update fixtures/compile expectations impacted by interface narrowing.

**Files to Create:**
- None.

**Tradeoffs:**
- **Split interfaces by workflow scope**: Chosen over keeping one broad backlog interface in review, because compile-time enforcement prevents dead/stubbed read methods in review adapters.
- **Keep existing broad `BacklogClient` for non-review flows**: Chosen over global refactor to avoid churn and regressions in refine/explore/status paths that depend on `List`/`Get`.

## Test Strategy

**Test Levels:**
1. **Unit tests (primary):** Validate review dependency typing/validation and adapter interface conformance.
2. **Workflow tests (existing acceptance-style tests):** Ensure review non-interactive behavior still creates backlog items and reports counts correctly.
3. **Focused package runs:** Execute review-targeted tests in `internal/pipeline` and `cmd/gromit`.

**Key Test Cases:**
- Review workflow compiles and runs with `BacklogWriter` dependency and still calls `Add` for backlog findings.
- Review dependency validation fails when backlog writer dep is nil with clear error text.
- `cliBacklogClient.Add` behavior remains unchanged (trimmed expected output text + required labels).
- Compile-time adapter assertion uses `pipeline.BacklogWriter` and no longer requires `List`/`Get`.

**Mocking Strategy:**
- Update review-only mocks in `internal/pipeline/review_test.go` to match writer surface where possible.
- Reuse existing tracker/learnings/log/state mocks to maintain end-to-end review coverage.
- Avoid changes to non-review backlog mocks unless needed for compilation.

**Coverage Goals:**
- Critical path: review parse -> backlog writer `Add` -> result counts/log/state.
- Guard path: nil review backlog writer dependency rejected early.
- Contract path: `cliBacklogClient` satisfies `BacklogWriter` at compile time.

**Test Organization:**
- Primary test files:
  - `internal/pipeline/review_test.go`
  - `cmd/gromit/review_test.go`
- Suggested commands during implementation:
  - `go test ./internal/pipeline -run Review`
  - `go test ./cmd/gromit -run 'Review|CliBacklogClient'`
  - `go test ./internal/pipeline ./cmd/gromit`

## Implementation Tasks

### Task 1: Introduce review-scoped `BacklogWriter` dependency in pipeline

**Files:**
- Modify: `internal/pipeline/pipeline.go`
- Modify: `internal/pipeline/review_test.go`

**What to Do:**
Define a new `BacklogWriter` interface with `Add` and `Update` methods in pipeline types. Add a review-specific dependency field in `Deps` for this interface and update `ReviewNonInteractive` plus `validateReviewDeps` to consume and validate the writer dependency. Update review tests/mocks and nil-dependency assertions to use the new field.

**Acceptance Criteria:**
- `BacklogWriter` interface exists with only `Add` and `Update`.
- `ReviewNonInteractive` no longer depends on full `BacklogClient`.
- Review dependency validation checks the writer dependency and corresponding tests pass.

**Dependencies:**
- None

**Notes:**
- Keep `BacklogClient` unchanged for refine/explore/status workflows.

### Task 2: Narrow `cliBacklogClient` in review CLI adapter

**Files:**
- Modify: `cmd/gromit/review.go`
- Modify: `cmd/gromit/review_test.go`

**What to Do:**
Switch review wiring to the new pipeline writer dependency, replace the compile-time check with `pipeline.BacklogWriter`, and remove `List`/`Get` from `cliBacklogClient`. Preserve `Add` semantics and existing `Update` behavior expected by the writer interface. Update tests that rely on the old interface shape or compile-time assertions.

**Acceptance Criteria:**
- `cliBacklogClient` has no `List` or `Get` methods.
- Compile-time check `var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)` is present.
- `runReviewNonInteractive` wires `cliBacklogClient` into the review writer dependency field.

**Dependencies:**
- Task 1

**Notes:**
- Ensure no regression in `TestCliBacklogClient_AddUsesTrimmedExpectedOutputs`.

### Task 3: Verify review-path behavior and run targeted quality gates

**Files:**
- Test: `internal/pipeline/review_test.go`
- Test: `cmd/gromit/review_test.go`

**What to Do:**
Run focused tests for review pipeline and CLI adapter paths, confirm no review code path calls removed methods, and verify all acceptance criteria from the spec are satisfied.

**Acceptance Criteria:**
- Review-targeted tests pass in both pipeline and CLI packages.
- No compile-time/runtime references to `cliBacklogClient.List` or `cliBacklogClient.Get` remain.
- Review workflow still reports created backlog items correctly.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
- Expand to broader package tests if targeted runs expose shared dependency breakage.

---

## Notes

- The interface narrowing is intentionally scoped to review. Other workflows continue to rely on `BacklogClient` read APIs.
- If dependency field naming in `pipeline.Deps` affects broader typed-interface tests, update only the minimal review-related expectations to keep churn low.
- After implementation, this plan is ready for decomposition into beads via `gromit decompose review-backlog-writer-interface`.
