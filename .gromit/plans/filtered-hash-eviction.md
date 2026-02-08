---
created: 2026-02-08T00:00:00Z
decomposed: true
decomposed_at: "2026-02-08T08:18:27-05:00"
id: filtered-hash-eviction
source_spec: filtered-hash-eviction
---

# Filtered Learning Hash Eviction Implementation Plan

**Goal:** Prune stale entries from `FilteredLearningHashes` in state.json by reconciling against current provisional learnings after each filter pass.

**Architecture:** Add a `ReconcileFilteredHashes` method on `state.File` that filters the hash slice to only retain hashes matching current provisionals, then wire it into `retro.Run()` after `FilterProvisional` completes with a single save.

**Tech Stack:** Go

**Spec:** `.gromit/specs/filtered-hash-eviction.md`

---

## Architecture

**Overview:**
A new `ReconcileFilteredHashes(currentHashes map[string]bool) bool` method on `state.File` prunes any hash from `FilteredLearningHashes` not present in the provided set. The caller in `retro.Run()` builds the current hash set from `GetProvisional()` after `FilterProvisional` has completed (so archived learnings are already removed), then calls reconcile before a single `Save()`.

**Integration Points:**
- `retro.Run()` already has both `stateFile` and `r.learningsFile` in scope at lines 91-116
- After `FilterProvisional`, `GetProvisional()` returns the post-filter provisional list
- `ReconcileFilteredHashes` prunes stale hashes, then a single `Save()` persists both additions and pruning

**Data Flow:**
1. `FilterProvisional` runs — archives generics, removes them from provisional list, returns newly evaluated hashes
2. `AddFilteredHashes` merges newly evaluated hashes into state
3. `GetProvisional()` returns the post-filter provisional list
4. Build `currentHashes` map from provisional `.Hash` fields
5. `ReconcileFilteredHashes(currentHashes)` prunes hashes not in the map
6. Single `Save()` if any changes occurred

**Files to Modify:**
- `internal/state/state.go` — Add `ReconcileFilteredHashes` method
- `internal/state/state_test.go` — Add unit tests
- `internal/retro/retro.go` — Wire reconciliation into `Run()` after the filter pass

## Test Strategy

**Unit Tests** (in `internal/state/state_test.go`):
- `TestReconcileFilteredHashes_PrunesStaleHashes`: Has hashes A,B,C,D — current set is {A,C} — result is [A,C], returns true
- `TestReconcileFilteredHashes_NoPruningNeeded`: All hashes match current set — returns false, slice unchanged
- `TestReconcileFilteredHashes_EmptyCurrentSet`: Prunes everything — returns true, slice empty
- `TestReconcileFilteredHashes_EmptyExistingHashes`: Nothing to prune — returns false
- `TestReconcileFilteredHashes_NilSafe`: Nil receiver doesn't panic

**Mocking Strategy:**
- No mocks needed — pure state manipulation, same pattern as existing `AddFilteredHashes` tests

## Implementation Tasks

### Task 1: Add ReconcileFilteredHashes method and tests

**Files:**
- Modify: `internal/state/state.go`
- Modify: `internal/state/state_test.go`

**What to Do:**
Add `ReconcileFilteredHashes(currentHashes map[string]bool) bool` to `state.File`. It filters `FilteredLearningHashes` in-place, keeping only hashes present in `currentHashes`. Returns `true` if any hashes were pruned. Include nil-receiver guard consistent with other methods.

Add unit tests covering: prunes stale hashes, no-op when all match, empty current set prunes all, empty existing hashes, nil receiver safety.

**Acceptance Criteria:**
- `ReconcileFilteredHashes` removes hashes not in the provided set and returns true when pruning occurred
- Nil receiver does not panic
- All existing `AddFilteredHashes` and persistence tests continue to pass

**Dependencies:** None

### Task 2: Wire reconciliation into retro.Run()

**Files:**
- Modify: `internal/retro/retro.go`

**What to Do:**
In `retro.Run()`, after `FilterProvisional` and `AddFilteredHashes`, collect current provisional hashes via `r.learningsFile.GetProvisional()`, build a `map[string]bool` from their `.Hash` fields, call `stateFile.ReconcileFilteredHashes(currentHashes)`, and adjust the save condition to trigger on either new hashes added or stale hashes pruned.

**Acceptance Criteria:**
- After `retro.Run()` completes, `FilteredLearningHashes` contains only hashes matching current provisional learnings
- State is saved once (not twice) when both additions and pruning occur
- No save occurs if there are no new hashes and no pruning needed

**Dependencies:**
- Task 1 (provides `ReconcileFilteredHashes` method)

---

## Notes

- The reconciliation naturally bounds `FilteredLearningHashes` size to the number of provisional learnings (typically under 10-20).
- The `GetProvisional()` call must happen after `FilterProvisional` so that archived learnings are already removed from the provisional list.
- The return bool from `ReconcileFilteredHashes` lets the caller avoid unnecessary writes when nothing changed.
