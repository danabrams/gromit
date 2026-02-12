# Investigation: Are precheck and scope check worth their cost?

**Date:** 2026-02-12
**Status:** Complete

## Symptom

Each bead iteration now requires 5-12+ Claude CLI invocations, up from the original 2 (build + validation). Two of these phases -- precheck and scope check -- each add a haiku invocation to every single iteration. The question: do they earn their per-bead cost?

## Evidence

### Data source

All JSONL logs in `.gromit/logs/` spanning Feb 5-11, 2026 (44 log files, 27 with iteration data).

### Precheck

| Metric | Value |
|--------|-------|
| Total beads processed (precheck ran) | 208 |
| Beads auto-closed by precheck (`precheck_skipped`) | 44 |
| Hit rate | 21.2% |
| Typical precheck duration | 13-28 seconds (avg ~22s) |
| Overhead on misses (164 beads x ~22s) | ~60 minutes |
| Savings on hits (44 beads x ~5-10 min saved) | ~220-440 minutes |
| **Net benefit** | **+160-380 minutes saved** |

Precheck finds criteria already met in roughly 1 in 5 beads. This typically happens when a prior bead's work incidentally satisfies a later bead's acceptance criteria (e.g., companion beads like "add method" + "add tests for method" where Claude wrote both). Each hit saves a full build + validation cycle (minutes), while each miss costs only ~22 seconds.

**Verdict: Precheck is net-positive. Keep it.**

### Scope check

| Metric | Value |
|--------|-------|
| Total beads where scope check ran | ~164 (all non-precheck-skipped) |
| Beads blocked by scope gate (`scope_blocked`) | **0** |
| Preemptive escalations triggered | **0** |
| Auto-escalations in buildPromptForBead | **0** |
| Typical scope check duration | ~15-20 seconds |
| Total overhead | ~45-55 minutes |
| **Total benefit** | **Zero** |

Scope check has three potential effects:
1. **Block oversized beads** before spending build compute -- never triggered
2. **Preemptive escalation** (medium -> high tier) in `setupBeadContext` -- never triggered
3. **Auto-escalation** (high complexity) in `buildPromptForBead` -- never triggered

Zero occurrences of any scope check action across the entire log history. The scope check has been pure overhead on every iteration since it was enabled.

**Verdict: Scope check provides zero value. Remove or make conditional.**

## Root Cause

**Precheck works** because beads in the same epic frequently have overlapping scope. When Claude completes bead A, it often incidentally satisfies bead B's acceptance criteria. Precheck catches this cheaply (~22s haiku) instead of running a full build cycle (~5-10 min).

**Scope check fails** because gromit's bead sizing rules (1-3 acceptance criteria, soft 4-5 file limit) already prevent over-scoped beads at creation time. By the time beads reach the runner, they're already well-scoped. The scope check is solving a problem that the planning pipeline already solved.

## Affected Code

- `internal/runner/runner.go:419-458` -- precheck invocation in main loop
- `internal/runner/runner.go:460-497` -- scope gate in main loop
- `internal/runner/runner.go:1550-1622` -- `runPrecheck` implementation
- `internal/runner/runner.go:1624-1681` -- `checkScope` implementation
- `internal/runner/process.go:103-108` -- preemptive escalation in `setupBeadContext`
- `internal/runner/process.go:114-147` -- `buildPromptForBead` with scope estimate
- `internal/config/config.go:68-72` -- `ScopeCheckConfig`
- `internal/config/config.go:74-79` -- `PrecheckConfig`

## Suggested Fix

### Recommendation: Keep precheck, disable scope check by default

**Precheck:** Keep as-is. 21% hit rate with significant time savings per hit. No changes needed.

**Scope check:** Change default to `enabled: false`. Three options in order of preference:

1. **Disable by default** (recommended) -- Change `SetDefaults()` to set `ScopeCheck.Enabled = false`. Users who decompose beads poorly can opt in. Zero code deletion needed, just a default change. One line in `config.go`.

2. **Make conditional** -- Only run scope check for beads with `complexity:high` label or P0 priority, where over-scoping risk is highest. Requires adding a condition in the main loop.

3. **Remove entirely** -- Delete `checkScope`, `ScopeCheckConfig`, the scope gate in the main loop, and the preemptive/auto-escalation logic. More invasive but cleaner. Would also remove the `scope_check_double_invocation_acceptance_test.go` and `scope_dedup_test.go` test files.

### If keeping scope check conditionally

If the concern is that removing scope check entirely loses a safety net for future use:

```yaml
scope_check:
  enabled: false  # Changed default from true to false
```

Users can re-enable it if they start seeing over-scoped beads. The code stays in place, just dormant.

## Impact

Disabling scope check saves ~15-20 seconds per bead iteration (one haiku invocation). Over a typical gromit run of 10-30 beads, that's 2.5-10 minutes saved per run. More importantly, it removes a network round-trip that can fail, adding latency and fragility to every iteration.
