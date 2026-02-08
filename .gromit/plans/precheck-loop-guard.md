---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T23:36:10-05:00"
id: precheck-loop-guard
source_spec: precheck-loop-guard
---

# Precheck Loop Guard Implementation Plan

**Goal:** Prevent infinite loops when precheck passes but Close fails or has no effect, by adding two complementary safety layers to the runner loop.

**Architecture:** Add failed-close beads to the existing `skippedBeads` map for immediate termination on retry, and track a consecutive precheck skip counter with a configurable limit as a catch-all safety valve.

**Tech Stack:** Go

**Spec:** `.gromit/specs/precheck-loop-guard.md`

---

## Architecture

**Overview:**
Two complementary safety mechanisms are added to the runner's precheck skip path: (1) add beads to `skippedBeads` when Close fails after precheck, leveraging the existing stuck-bead termination logic, and (2) track consecutive precheck skips with a configurable limit that stops the loop with an error.

**Key Components:**
1. **`MaxConsecutiveSkips` config field** (`LoopConfig`): New integer field mirroring `StuckBeadThreshold` pattern, with default of 3.
2. **`skippedBeads` integration**: On Close failure in the precheck path, add bead ID to the existing `skippedBeads` map so the stuck-bead termination logic kicks in on the next iteration.
3. **Consecutive skip counter**: A local counter in `Run()` that increments on each precheck skip and resets to zero on any real build iteration. When it hits the limit, the loop returns an error.

**Integration Points:**
- `internal/config/config.go` — `LoopConfig` struct gets one new field, `setDefaults()` gets one new default
- `internal/runner/runner.go` — `Run()` method gets ~15 lines of new logic in the precheck skip path + counter reset after `processBead`

**Data Flow:**
1. Bead fetched via `Ready()` → precheck runs → if passed:
   - `Close()` called → if error: bead added to `skippedBeads`, consecutive counter incremented
   - `Close()` succeeds → consecutive counter incremented
   - If counter >= `maxConsecutiveSkips` → return error
2. If bead goes to real build → counter reset to 0

**Files to Modify:**
- `internal/config/config.go` — Add `MaxConsecutiveSkips int` to `LoopConfig`, add default in `setDefaults()`
- `internal/runner/runner.go` — Add counter variable, add `skippedBeads` on Close failure, add limit check, add counter reset

**Tradeoffs:**
- **Counter scope is per-run, not per-bead**: A global consecutive counter is simpler and catches both single-bead and multi-bead spinning.
- **Error return vs. break**: Using `return fmt.Errorf(...)` rather than `break` ensures non-zero exit code, matching the spec's "stop with error" requirement.

## Test Strategy

**Test Levels:**
1. **Unit Tests (config)**: Verify `MaxConsecutiveSkips` default and YAML parsing
2. **Unit Tests (runner)**: Verify both loop guard behaviors via mock-based integration tests

**Key Test Cases:**

*Config tests (`internal/config/config_test.go`):*
- `MaxConsecutiveSkips` defaults to 3 when not set
- `MaxConsecutiveSkips` is loaded from YAML when explicitly set
- `MaxConsecutiveSkips` of 0 defaults to 3

*Runner tests (`internal/runner/interfaces_test.go`):*
- Close failure after precheck → bead added to `skippedBeads` → loop terminates
- Consecutive skip limit reached → loop stops with error
- Counter reset on real build iteration → no premature termination
- Custom limit from config → loop stops at configured threshold

**Mocking Strategy:**
- Use existing `mockBeadClient` with `CloseFn` for Close failures
- Use existing `mockClaudeClient` with `RunFn` for `PRECHECK_PASSED`
- Control `ReadyFn` call count for repeated/different beads

**Test Organization:**
- Config tests in `internal/config/config_test.go` (table-driven)
- Runner tests in `internal/runner/interfaces_test.go` (matching `TestRunWithMocks_Precheck*` pattern)

## Implementation Tasks

### Task 1: Add MaxConsecutiveSkips to config

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `MaxConsecutiveSkips int` field to `LoopConfig` struct with yaml tag `max_consecutive_skips`. Add default of 3 in `setDefaults()` when the value is 0, following the exact pattern of `StuckBeadThreshold` (line 229-230). Add table-driven tests verifying the default, explicit YAML value, and zero-defaults-to-3 behavior.

**Acceptance Criteria:**
- `MaxConsecutiveSkips` defaults to 3 when not set in config
- `MaxConsecutiveSkips` is correctly parsed from YAML
- Zero value defaults to 3

**Dependencies:**
- None (foundational)

**Notes:**
Follow the `StuckBeadThreshold` pattern exactly — same struct, same default logic, same test style.

### Task 2: Add skippedBeads on Close failure after precheck

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/interfaces_test.go`

**What to Do:**
In the precheck skip path (around line 326-328), when `Close()` returns an error, add the bead ID to the `skippedBeads` map. The existing stuck-bead detection logic at lines 297-318 will then catch the bead on the next loop iteration and terminate. Add a test where precheck passes, Close returns an error, Ready returns the same bead again, and verify the loop terminates with the "All ready beads are stuck" message.

**Acceptance Criteria:**
- When Close fails after precheck pass, bead ID is added to `skippedBeads`
- On next iteration, if Ready returns the same bead, loop terminates cleanly

**Dependencies:**
- None (independent of Task 1)

**Notes:**
The `skippedBeads` check at line 299 already handles the "all ready beads are stuck" termination — this task just ensures precheck Close failures feed into that mechanism.

### Task 3: Add consecutive precheck skip counter and limit

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/interfaces_test.go`

**What to Do:**
Add a `consecutiveSkips` counter variable initialized to 0 alongside `iteration` (line 229). In the precheck skip path, increment the counter after the Close/Sync calls. After incrementing, check if `consecutiveSkips >= cfg.Loop.MaxConsecutiveSkips` — if so, return an error like `fmt.Errorf("stopped: %d consecutive precheck skips without any real build work (possible loop — check bd state)")`. After `processBead` (around line 384), reset `consecutiveSkips = 0`. Add tests for: limit reached (3 consecutive skips → error), counter reset (skip, build, skip → no error), and custom limit (set to 2 → stops at 2).

**Acceptance Criteria:**
- Consecutive precheck skips without real build work trigger loop stop with error
- A real build iteration resets the counter to zero
- The limit uses `cfg.Loop.MaxConsecutiveSkips`

**Dependencies:**
- Task 1 (needs `MaxConsecutiveSkips` in config)

---

## Notes

- The precheck skip path currently does not increment the `iteration` counter — this is intentional and preserved. The consecutive skip counter is a separate safety mechanism.
- Both layers are complementary: Layer 1 (skippedBeads) catches Close errors on the 2nd spin. Layer 2 (consecutive counter) catches the subtler case where Close returns nil but doesn't work.
- All changes are confined to two files (`config.go` and `runner.go`) plus their test files.
