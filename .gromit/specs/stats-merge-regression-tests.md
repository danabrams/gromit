---
id: stats-merge-regression-tests
source_ideas: []
created: 2026-02-26
epic: test-quality
---

# Global Stats Merge Regression Tests

## Problem

The existing `TestOrchestrator_MergesGlobalStatsPreservingExistingData` test covers the basic merge case but misses edge cases: empty initial stats, multiple model entries merging independently, and zero-value fields that should not overwrite non-zero existing values. These gaps leave accumulation bugs undetectable.

## Approach

- Extend `TestOrchestrator_MergesGlobalStatsPreservingExistingData` with subtests (using `t.Run`) for each edge case
- Subtest: **empty initial stats** — start with zero-value stats struct, merge one run's output, verify all fields are set correctly
- Subtest: **multiple model entries** — stats contain entries for two different models; verify each model's counts accumulate independently without cross-contamination
- Subtest: **zero-value fields** — new run reports zero for a field that has a non-zero existing value; verify the existing non-zero value is preserved (not overwritten with zero), or document the intended overwrite semantics if that is correct
- Tests should use in-memory structs only, no filesystem I/O

## Files to Change

- `internal/runner/orchestrator_test.go` — extend `TestOrchestrator_MergesGlobalStatsPreservingExistingData` with `t.Run` subtests

## Acceptance Criteria

- Subtest for empty initial stats passes, verifying fields are populated from the first merge
- Subtest for multiple model entries verifies per-model independence
- Subtest for zero-value fields documents and enforces the intended merge semantics
- All subtests are deterministic with no sleep or timing dependencies
- `go test ./internal/runner/...` passes with all subtests
