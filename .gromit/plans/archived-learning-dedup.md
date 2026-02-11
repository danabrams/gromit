---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T07:12:35-05:00"
id: archived-learning-dedup
source_spec: archived-learning-dedup
---

# Archived Learning Dedup Implementation Plan

**Goal:** Prevent duplicate entries in the archived section by adding a hash dedup check in `Add()` before the LLM filter call.

**Architecture:** Add a `for _, l := range f.archived` hash check loop in `Add()` after the existing confirmed/provisional checks, returning `(nil, nil)` on match — the same "silently skip" pattern used for the other sections.

**Tech Stack:** Go

**Spec:** `.gromit/specs/archived-learning-dedup.md`

---

## Architecture

**Overview:**
Add a hash dedup check against `f.archived` in `Add()`, positioned after the existing confirmed/provisional checks (line 128) and before the filter call (line 131). This follows the identical pattern already used for confirmed and provisional dedup.

**Key Components:**
1. **`Add()` in `internal/learnings/learnings.go`**: Add a `for _, l := range f.archived` hash check loop returning `(nil, nil)` on match — identical to lines 119-127.

**Integration Points:**
- Slots into the existing dedup check block at lines 119-128, extending it with one more section
- No new types, interfaces, or return values needed
- No changes to `FilterProvisional()`, `Save()`, or any other method

**Files to Modify:**
- `internal/learnings/learnings.go` — Add archived hash check loop in `Add()` (4 lines)
- `internal/learnings/learnings_test.go` — Add test cases for archived dedup

**Tradeoffs:**
- Check order (confirmed → provisional → archived → filter): Keeps archived last among hash checks since it's the least likely match path, but still before the LLM filter call which is the expensive operation to avoid.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Direct tests of `Add()` behavior when archived duplicates exist

**Key Test Cases:**
- **Archived duplicate skipped**: Pre-populate `f.archived` with a learning, call `Add()` with the same content, assert `(nil, nil)` returned and no state changes
- **Archived duplicate skips filter**: Set a filter func with a `filterCalled` bool, pre-populate archived, call `Add()` with matching content, assert filter was never called
- Existing confirmed/provisional dedup tests already cover regression — no changes needed

**Mocking Strategy:**
- Uses the same pattern as `TestFilterFuncCalledAfterDuplicateCheck`: a closure with a `filterCalled` bool to verify the filter is not invoked
- No external mocks needed — `File` is self-contained with in-memory state

**Test Organization:**
- New tests in `internal/learnings/learnings_test.go` following existing `TestAdd*` naming conventions

## Implementation Tasks

### Task 1: Add archived hash dedup check and tests

**Files:**
- Modify: `internal/learnings/learnings.go`
- Modify: `internal/learnings/learnings_test.go`

**What to Do:**
Add a `for _, l := range f.archived` hash check loop in `Add()` after line 127 (the provisional dedup check), returning `(nil, nil)` on match. Add two test cases: one verifying `(nil, nil)` return with no state changes, one verifying the filter function is never called.

**Acceptance Criteria:**
- `Add()` returns `(nil, nil)` without calling the filter function when the new learning's content hash matches an existing archived learning's hash
- `Add()` does not call `Save()` when skipping an archived duplicate
- Existing confirmed/provisional dedup tests still pass

**Dependencies:** None

---

## Notes

- The implementation is a 4-line addition mirroring the exact pattern at lines 119-127 of `learnings.go`.
- The archived content includes `*Archived from ...*` suffix text appended by the `Archive()` method and the inline filter, but `hashContent()` is called on the original content at add-time and stored in the `Hash` field — so the hash comparison works correctly regardless of the archived reason text.
