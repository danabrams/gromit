---
id: decompose-cancellation-test
source_ideas: []
created: 2026-02-26
epic: test-quality
---

# Decompose Cancellation-Path Test

## Problem

The decompose workflow's cancellation path has no deterministic test. Existing scaffolding uses skip-based placeholders or relies on timing, making it impossible to reliably assert that `TimeoutDecompositionReason` contains the expected "parent context canceled" message when decomposition is triggered on an already-cancelled context.

## Approach

- Use `context.WithCancel` to create a context, cancel it immediately before passing to the decompose path, and assert that the returned `TimeoutDecompositionReason` string contains "parent context canceled" (or the actual sentinel string used in the implementation)
- Find the relevant function under test in `internal/runner/` (likely the decomposition handler or a helper called from `CheckProactiveDecomposition`)
- Write the test to invoke that function with a pre-cancelled context and assert on the reason string
- No sleep or timing dependencies; the test must pass deterministically under `go test -count=5`
- Remove or replace any existing skip-based placeholder that was deferring this coverage

## Files to Change

- `internal/runner/proactive_decompose_test.go` (or wherever the decompose tests live) — add `TestDecomposeCancellationPath` using `context.WithCancel`

## Acceptance Criteria

- Test uses `context.WithCancel`, cancels the context before invocation, and asserts on the returned reason string
- Reason string contains "canceled" (matching the actual sentinel)
- Test passes deterministically with `go test -count=5 ./internal/runner/...`
- No `t.Skip` calls in the new test
- Any pre-existing skip-based placeholder for this case is removed
