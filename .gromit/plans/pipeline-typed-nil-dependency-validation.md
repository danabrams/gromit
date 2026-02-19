---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T18:16:14Z"
id: pipeline-typed-nil-dependency-validation
source_spec: pipeline-typed-nil-dependency-validation
---

# Pipeline Typed Nil Dependency Validation Implementation Plan

**Goal:** Replace map+interface nil-check pattern in `validateReviewDeps` and `validateExploreDeps` with explicit typed nil checks that catch typed nil values.

**Architecture:** Sequential `if p.deps.X == nil` checks on each dependency field, preserving existing error strings and first-missing-returns-first semantics.

**Tech Stack:** Go

**Spec:** `.gromit/specs/pipeline-typed-nil-dependency-validation.md`

---

## Architecture

Both validators currently build a `map[string]interface{}` of required dependencies and loop with `dep == nil`. This misses typed nil values (e.g. `(*concreteImpl)(nil)` stored in an interface field evaluates as `!= nil` in Go). The fix replaces each validator's body with sequential explicit nil checks on the typed struct fields. The `p.deps == nil` guard at the top of each function is preserved.

- `validateReviewDeps` checks 7 fields: ClaudeClient, PromptRenderer, BeadClient, BacklogClient, LearningsManager, LogWriter, StateManager
- `validateExploreDeps` checks 3 fields: AgentResolver, PromptRenderer, BacklogClient

No new types, files, or abstractions. Error messages unchanged.

## Test Strategy

Table-driven tests for each validator covering:
- Each dependency nil individually → correct error string
- Typed nil dependency (concrete nil pointer assigned to interface field) → treated as missing
- All deps present → nil error

Use existing mock types from pipeline test files. Typed nil cases use `(*mockType)(nil)` cast to the interface field.

## Implementation Tasks

### Task 1: Rewrite validateReviewDeps with explicit typed nil checks

**Files:**
- Modify: `internal/pipeline/pipeline.go`
- Test: `internal/pipeline/review_test.go`

**What to Do:**
Replace the `map[string]interface{}` + loop body of `validateReviewDeps` (lines 352-366) with 7 sequential `if p.deps.X == nil` checks, each returning `fmt.Errorf("pipeline: nil %s", name)` with the same error text. Keep the `p.deps == nil` guard. Add table-driven test covering each nil dependency individually, a typed nil case (e.g. `ClaudeClient: (*testClaudeClient)(nil)`), and an all-present success case.

**Acceptance Criteria:**
- `validateReviewDeps` uses direct nil checks per field, no map or loop.
- Typed nil `ClaudeClient` returns `"pipeline: nil ClaudeClient"`.
- Existing error messages unchanged for all 7 dependencies.

**Dependencies:** None

### Task 2: Rewrite validateExploreDeps with explicit typed nil checks

**Files:**
- Modify: `internal/pipeline/explore.go`
- Test: `internal/pipeline/explore_test.go`

**What to Do:**
Replace the `map[string]interface{}` + loop body of `validateExploreDeps` (lines 135-145) with 3 sequential `if p.deps.X == nil` checks. Keep the `p.deps == nil` guard. Extend existing `TestPipeline_ExploreValidatesDeps` with a typed nil test case (e.g. `AgentResolver: (*testAgentResolver)(nil)`) and an all-present success case.

**Acceptance Criteria:**
- `validateExploreDeps` uses direct nil checks per field, no map or loop.
- Typed nil `AgentResolver` returns `"pipeline: nil AgentResolver"`.
- Existing error messages unchanged for all 3 dependencies.

**Dependencies:** None (independent of Task 1)

---

## Notes

- Tasks 1 and 2 are independent and can be decomposed into parallel beads.
- The check order within each validator should match the field order in the `Deps` struct for readability.
- Neither validator should use `reflect` — plain `== nil` on typed interface fields is sufficient because Go compares the interface value directly (not wrapped in another interface).
