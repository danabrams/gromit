# Codebase Health Analysis — 2026-02-21

## Symptoms Reported

1. Test suites are becoming very slow
2. Experiencing regressions
3. A long, never-ending backlog

## Root Cause: One Loop, Three Symptoms

All three symptoms share a single root cause: **the system generates complexity faster than it can digest it.** Gromit is a self-improving loop that produces its own backlog by design. The review phase creates `[from-review]` beads; the retro creates learnings that become requirements; the decomposition phase splits beads into more beads. The generation rate matches or exceeds the completion rate. This creates permanent backlog pressure, which drives constant additions to the Runner, which causes regressions, which require more tests, which slow the suite.

## Measured State (2026-02-21)

| Metric | Value |
|---|---|
| Total production files | 203 |
| Total test files | 414 (2:1 ratio) |
| Runner package test files | 171 |
| Runner `_reclassified_test.go` files | 13 |
| `t.Skip` violations (rules prohibit) | 76 |
| `runner_test.go` size | 3,956 lines (with `//go:build acceptance`) |
| Runner struct fields | ~40 |
| Files over 550-line production limit | 5+ |
| Total open beads | 144 |
| Total closed beads | 1,457 |
| Specs | 118 |

### Files Violating 550-Line Rule

| File | Lines | % Over Limit |
|---|---|---|
| `internal/logger/process_trend.go` | 1,357 | 147% over |
| `internal/runner/process_methodology.go` | 774 | 41% over |
| `internal/runner/callbacks.go` | 756 | 37% over |
| `internal/runner/lifecycle.go` | 655 | 19% over |
| `internal/runner/escalation/handler.go` | 529 | 4% over |

## Problem 1: Slow Tests

- `runner_test.go` is 3,956 lines with `//go:build acceptance` — real subprocess machinery in every test
- 76 `t.Skip` calls mask broken tests instead of deleting them — the suite runs but doesn't verify
- 171 test files in a single package creates compilation overhead
- Acceptance tests mixed with unit tests: acceptance budget rule (6,000 lines) is being consumed

## Problem 2: Regressions

- The `Runner` struct has ~40 fields spread across 20+ files
- `callbacks.go` (756 lines) is exercised by every execution path — changes have unbounded blast radius
- 13 `_reclassified_test.go` files are scar tissue from structural shifts: tests broke when code moved, were reclassified instead of deleted or restructured
- `runner_split_phase1/2/3/4` test artifacts remain from an incomplete split attempt

## Problem 3: Never-Ending Backlog

The backlog is **infinite by design**: review generates beads, retro generates requirements, decomposition multiplies beads. With 1,457 closed and 144 open, the backlog is not converging. This creates permanent pressure to add rather than stabilize, which feeds back into Problems 1 and 2.

## Proposed Fixes (Priority Order)

### Immediate Cleanup (days)

1. **Treat `t.Skip` as deletion** — 76 violations. Each one is a hidden regression. Remove or fix. Beads already exist: gromit-g7q9, gromit-wy62, gromit-yhvc, gromit-cptlj.
2. **Delete `_reclassified_test.go` and `_split_phase*` test artifacts** — 13+ files. These are archaeological noise. Evaluate each: delete if redundant, merge if unique coverage.

### Structural Refactor (weeks)

3. **Hard-line the Runner to a 5-stage pipeline** — each stage becomes a separate type with its own package. The Runner becomes a thin orchestrator with no logic. This limits blast radius for changes. Without this, every new methodology or provider attaches itself to the Runner and regressions continue.

### Systemic Change (ongoing)

4. **Freeze new feature work during cleanup** — reviews should keep creating beads (that's the system working as intended), but new feature specs should pause until the cleanup pass is complete. The open count should trend down before trending up again.

## On Rebuilding

The user raised the option of rebuilding from core learnings and specifications.

**A rebuild would genuinely help if:**
- The specs, rules, and architectural concepts are carried forward
- The new runner is a bounded pipeline (5-6 files max), each stage independently testable
- A hard "no new features until tests are green and fast" rule is imposed from day one

**A rebuild will recreate the same state in 6-12 months if:**
- The same incremental bead-by-bead development methodology is used
- The review/retro loop generates backlog at the same rate without a cap
- The Runner is allowed to accumulate methods rather than delegate to stages

**The real architectural question:** What structural constraint prevents the God Object from forming again? Every new methodology and provider naturally attaches to the Runner. Without a mechanical constraint that makes direct attachment impossible, the same accumulation will happen.

## Key Architectural Insight

The Runner was designed as a coordinator but keeps absorbing implementation. Sub-package splits (escalation/, methodology/, tdd/, validation/) helped but didn't fully decouple — because the Runner struct still owns all state and the parent package still imports everything. The fix is not another split; it's a different object model where stages receive inputs and return outputs, with the Runner holding no state beyond what's needed to wire stages together.

## Next Discussion Topics

- Should the rebuild option be explored with a specific architectural design (pipeline, event-driven, etc.)?
- What is the acceptable steady-state for the backlog, and how is it enforced?
- Is the current rate of spec creation (118 specs, adding constantly) sustainable, or is there a spec review/deprecation process needed?
