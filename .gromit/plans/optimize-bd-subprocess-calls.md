---
created: 2026-02-12T00:00:00Z
decomposed: true
decomposed_at: "2026-02-12T15:15:10-05:00"
id: optimize-bd-subprocess-calls
source_spec: optimize-bd-subprocess-calls
---

# Optimize bd Subprocess Calls Implementation Plan

**Goal:** Eliminate redundant and inefficient bd subprocess calls in the runner's main loop — cache parent bead per iteration, use targeted `--parent` queries, and reduce Ready batch size.

**Architecture:** Fetch parent once in the main loop and pass it as a parameter to `runPrecheck`, `checkScope`, and `setupBeadContext`. Replace `HasOpenChildren`'s full list scan with `bd list --parent <id> --limit 1`. Reduce `Ready` limit from 10 to 3.

**Tech Stack:** Go

**Spec:** `.gromit/specs/optimize-bd-subprocess-calls.md`

---

## Architecture

**Overview:**
Three targeted optimizations to reduce redundant bd subprocess forks during each iteration. No behavioral changes — only performance characteristics differ.

**Integration Points:**
- `runPrecheck()`, `checkScope()`, `setupBeadContext()` gain a `parent *bead.Bead` parameter
- `HasOpenChildren()` and `Ready()` change internally only — no signature changes
- `DecomposeTask()` keeps its own `GetParent` call (separate code path via `attemptDecomposition`)
- Main loop fetches parent once after `getNextBead()` when `b.Parent != ""`

**Data Flow (main loop iteration):**
1. `getNextBead()` returns bead `b`
2. If `b.Parent != ""`, call `GetParent(b)` once → `parent`
3. Pass `parent` to `runPrecheck(ctx, b, parent)` → uses it for PrecheckContext
4. Pass `parent` to `checkScope(ctx, b, parent)` → uses it for ScopeContext
5. Pass `parent` to `setupBeadContext(ctx, b, iteration, deadline, scopeEstimate, parent)` → stores in beadContext

**Files to Modify:**
- `internal/bead/bead.go` — `HasOpenChildren` implementation, `Ready` limit
- `internal/runner/runner.go` — Main loop parent fetch, `runPrecheck` and `checkScope` signatures
- `internal/runner/process.go` — `setupBeadContext` signature

**Tradeoffs:**
- **Parameter passing over lazy cache** — Explicit, scoped to iteration, no stale-data risk, no cache invalidation logic
- **Keep DecomposeTask independent** — It's on a separate code path (epic decomposition in process.go:563), threading parent through more layers for a single call isn't worth the coupling
- **`--limit 1` for HasOpenChildren** — Only needs boolean answer, `--limit 0` would serialize all matching children
- **Reduce Ready to 3, not 1** — `--limit 1` would break epic filtering if the single result is an epic

## Test Strategy

**Unit Tests (`internal/bead/bead_test.go`):**
- Verify `HasOpenChildren` passes `--parent <id>`, `--status open`, `--limit 1` to bd
- Verify `HasOpenChildren` returns true on non-empty result, false on empty
- Verify `Ready` uses `--limit 3`
- Existing epic-filtering tests continue to pass

**Unit Tests (`internal/runner/`):**
- Verify `runPrecheck`, `checkScope`, `setupBeadContext` use passed parent (no `GetParent` call)
- Verify main loop calls `GetParent` at most once per iteration (mock call counting)
- Verify `nil` parent passed when bead has no parent

**Mocking Strategy:**
- Existing `mockBeadClient.GetParentFn` with call counting
- No new mock types needed

**Coverage Goals:**
- All existing tests pass without modification (behavioral equivalence)
- Command argument verification for HasOpenChildren and Ready changes

## Implementation Tasks

### Task 1: Optimize HasOpenChildren to use `--parent` flag

**Files:**
- Modify: `internal/bead/bead.go`
- Test: `internal/bead/bead_test.go`

**What to Do:**
Replace the `HasOpenChildren` implementation at bead.go:689. Currently it calls `c.List()` which runs `bd list --json --status open --sort priority --limit 0` and iterates all results client-side. Replace with a targeted query: `c.run("list", "--json", "--status", "open", "--parent", parentID, "--limit", "1")`. Parse the output — if at least one bead is returned, return true. The `--parent` flag is already used in bead.go:375 (CreateWithParent), confirming bd supports it.

**Acceptance Criteria:**
- `HasOpenChildren` runs `bd list --json --status open --parent <id> --limit 1` instead of fetching all open beads
- Returns true when bd returns a non-empty array, false when empty
- Existing validation (nil client, invalid parentID) still works

**Dependencies:** None

### Task 2: Reduce Ready batch limit from 10 to 3

**Files:**
- Modify: `internal/bead/bead.go`

**What to Do:**
Change line 195 from `c.run("ready", "--json", "--limit", "10")` to `c.run("ready", "--json", "--limit", "3")`. The `parseBeadOutputExcluding` logic is unchanged — it still iterates the batch looking for the first non-epic.

**Acceptance Criteria:**
- `Ready()` runs `bd ready --json --limit 3` instead of `--limit 10`
- All existing Ready tests pass (epic filtering still works with smaller batch)

**Dependencies:** None

### Task 3: Cache parent bead per iteration in the main loop

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/process.go`
- Test: existing runner test files (signature updates propagate through mocks)

**What to Do:**

1. **Main loop (runner.go ~line 428):** After `getNextBead()` returns a bead and before `runPrecheck`, add parent fetching:
   ```go
   var parent *bead.Bead
   if b.Parent != "" {
       var err error
       parent, err = r.beads.GetParent(b)
       if err != nil {
           r.log("Warning: failed to get parent bead: %v", err)
       }
   }
   ```
   Pass `parent` to `runPrecheck(ctx, b, parent)`, `checkScope(ctx, b, parent)` (via `r.checkScope`), and to `processBead` → `setupBeadContext`.

2. **`runPrecheck` (runner.go:1607):** Change signature to `runPrecheck(ctx context.Context, b *bead.Bead, parent *bead.Bead) (bool, time.Duration)`. Remove the internal `GetParent` call at line 1620. Use the `parent` parameter directly for `PrecheckContext.ParentBead`.

3. **`checkScope` (runner.go:1680):** Change signature to `checkScope(ctx context.Context, b *bead.Bead, parent *bead.Bead) *prompt.ScopeEstimate`. Remove the internal `GetParent` call at line 1686. Use the `parent` parameter directly for `ScopeContext.ParentBead`.

4. **`setupBeadContext` (process.go:56):** Add `parent *bead.Bead` parameter to signature. Remove the internal `GetParent` call at line 83. Use the passed `parent` directly when building the `beadContext`.

5. **`processBead` (runner.go:698):** Thread the parent parameter through to `setupBeadContext`. Either add parent to `processBead`'s signature, or pass it via the existing `scopeEstimate` pattern (add to the call chain).

6. **Update callers:** All call sites of `runPrecheck`, `checkScope`, `setupBeadContext`, and `processBead` must be updated to pass the parent parameter.

**Acceptance Criteria:**
- `GetParent` is called at most once per iteration for any given bead, regardless of which phases execute
- The parent is fetched in the main loop only when `b.Parent != ""`
- All existing tests pass (mock `GetParentFn` calls still work — they just get called once from the main loop instead of 3 times from individual methods)

**Dependencies:** None (independent of Tasks 1 and 2, but largest change so listed last)

---

## Notes

- All three tasks are independent and can be implemented in parallel or any order
- Task 3 is the largest and touches the most files — it changes function signatures across runner.go and process.go
- The `DecomposeTask` method (runner.go:1405) keeps its own `GetParent` call since it's on a separate code path (epic decomposition) and doesn't run in the same iteration flow as precheck/scope/setup
- Existing test mocks already have `GetParentFn` fields — the signature changes just mean the mock function gets called from a different location (main loop instead of individual methods)
